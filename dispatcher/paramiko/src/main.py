from auth import auth_user
from connector import connect_server
from session import handler
from utils import log_event, resource_manager
from reader import line_reader
import logging
import socket
import threading
import paramiko
import time

logging.basicConfig(
  level=logging.WARNING,
  format="%(asctime)s [%(levelname)s] %(name)s - %(message)s",
)

logger = logging.getLogger(__name__)

HOST = "0.0.0.0"
PORT = 22

HOST_KEY = paramiko.RSAKey(filename="/certs/ssh_host_rsa_key")
COWRIE_VERSION = None

class SSHProxyServer(paramiko.ServerInterface):
  def __init__(self, client_addr):
    self.event = threading.Event()
    self.username = None
    self.password = None
    self.authenticator = auth_user.Authenticator()
    self.heralding_connector = connect_server.SSHConnector(host="heralding")
    self.cowrie_connector = connect_server.SSHConnector(host="cowrie", port=2222)
    self.client_addr = client_addr
    self.is_exec_request = False
    self.request_type = None
    self.exec_command = None

  def check_auth_password(self, username: str, password: str) -> int:
    self.username = username
    self.password = password

    try:
      self.heralding_connector.record_login(username=username, password=password)
    except Exception:
      logger.exception("Failed to record login via heralding_connector")

    auth_success = self.authenticator.authenticate(username, password)
    log_event.log_auth_event(self.client_addr, HOST, PORT, username, password, auth_success)

    return paramiko.AUTH_SUCCESSFUL if auth_success else paramiko.AUTH_FAILED

  def check_channel_request(self, kind: str, chanid: int) -> int:
    if kind == "session":
      return paramiko.OPEN_SUCCEEDED
    return paramiko.OPEN_FAILED_ADMINISTRATIVELY_PROHIBITED

  def check_channel_pty_request(self, channel, term, width, height, pixelwidth, pixelheight, modes) -> bool:
    return True

  def check_channel_shell_request(self, channel) -> bool:
    self.request_type = "shell"
    self.event.set()
    return True

  def check_channel_exec_request(self, channel, command) -> bool:
    self.is_exec_request = True
    self.request_type = "exec"
    self.exec_command = command
    self.event.set()
    threading.Thread(
      target=self._handle_exec_request,
      args=(channel, command),
      daemon=True
    ).start()
    return True

  def _handle_exec_request(self, channel, command):
    """exec requestを処理（ssh user@host "command"形式）"""
    try:
      command_str = command.decode('utf-8', errors='ignore')

      try:
        src_ip, src_port = channel.getpeername()
      except:
        src_ip, src_port = "unknown", 0

      log_event.log_command_event(src_ip, src_port, self.username, command_str, "~")

      try:
        output = self.cowrie_connector.execute_command_via_shell(
          command_str,
          self.username,
          self.password
        )
        channel.send(output.encode('utf-8'))
        channel.send_exit_status(0)
      except Exception:
        logger.exception("Failed to execute command on cowrie")
        channel.send(b"Command execution failed.\n")
        channel.send_exit_status(1)

    except Exception:
      logger.exception("Error in _handle_exec_request")
      channel.send_exit_status(1)
    finally:
      try:
        reader = line_reader.LineReader(channel, self.username, self.password)
        reader.cleanup_terminal()
      except Exception:
        logger.exception("Failed to cleanup terminal (exec)")

      resource_manager.close_channel(channel)

def _handle_client(client, addr):
  transport = None
  chan = None
  session_started = False
  try:
    logger.info("Connection from %s", addr)

    client.setsockopt(socket.IPPROTO_TCP, socket.TCP_CORK, 1)

    transport = paramiko.Transport(client)

    transport.add_server_key(HOST_KEY)

    if COWRIE_VERSION:
      transport.local_version = COWRIE_VERSION
      logger.debug("Set server version to: %s", COWRIE_VERSION)

    security_opts = transport.get_security_options()
    security_opts.ciphers = ("aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-cbc", "aes192-cbc", "aes256-cbc")
    security_opts.digests = ("hmac-sha2-256", "hmac-sha2-512", "hmac-sha1")
    security_opts.key_types = ("rsa-sha2-512", "rsa-sha2-256", "ssh-rsa")
    security_opts.kex = ("ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521", "diffie-hellman-group-exchange-sha256", "diffie-hellman-group14-sha256", "diffie-hellman-group16-sha512", "diffie-hellman-group14-sha1")
    security_opts.compression = ("none",)

    server = SSHProxyServer(addr)

    try:
      transport.start_server(server=server)
      client.setsockopt(socket.IPPROTO_TCP, socket.TCP_CORK, 0)
    except paramiko.SSHException:
      logger.warning("SSH negotiation failed")
      return
    except EOFError:
      logger.info("Client closed connection during handshake (EOF)")
      return
    except Exception:
      logger.exception("Unexpected error during SSH handshake")
      return

    chan = transport.accept(20)
    if chan is None:
      logger.warning("No channel")
      return

    try:
      server.event.wait(timeout=1.0)
    except Exception:
      pass

    if server.is_exec_request:
      session_started = True
      return

    username = server.username
    password = server.password
    start_time = time.time()

    threading.Thread(
      target=handler.handle_session,
      args=(chan, username, password, addr, start_time, server.cowrie_connector),
      daemon=True
    ).start()

    session_started = True

  except EOFError:
    logger.info("Client closed connection after authentication (EOF)")
  except Exception:
    logger.exception("Error accepting connection")
  finally:
    if not session_started:
      try:
        if chan is not None:
          resource_manager.close_channel(chan)
      except Exception:
        logger.exception("Failed to close channel in finally (no session)")
      resource_manager.close_proxy_connection(transport=transport, client=client)


def start_proxy():
  global COWRIE_VERSION

  COWRIE_VERSION = connect_server.fetch_server_version("cowrie", 2222)
  logger.info("Using SSH version string: %s", COWRIE_VERSION)

  sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
  sock.bind((HOST, PORT))
  sock.listen(100)
  logger.info("SSH Proxy listening on %s:%s", HOST, PORT)

  try:
    while True:
      try:
        client, addr = sock.accept()
      except Exception:
        logger.exception("Socket accept failed")
        continue

      threading.Thread(target=_handle_client, args=(client, addr), daemon=True).start()
  except Exception:
    logger.exception("Fatal error in accept loop")
  finally:
    try:
      sock.close()
    except Exception:
      logger.exception("Failed to close listening socket")

if __name__ == "__main__":
  start_proxy()
