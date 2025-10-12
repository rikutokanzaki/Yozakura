from connector import connect_server
from session import handler
from utils import log_event
import socket
import threading
import paramiko
import time

HOST = "0.0.0.0"
PORT = 22

HOST_KEY = paramiko.RSAKey(filename="/certs/ssh_host_rsa_key")

class SSHProxyServer(paramiko.ServerInterface):
  def __init__(self, client_addr):
    self.event = threading.Event()
    self.username = None
    self.password = None
    self.cowrie_connector = connect_server.SSHConnector(host="cowrie", port=2222)
    self.client_addr = client_addr

  def check_auth_password(self, username, password):
    self.username = username
    self.password = password

    try:
      self.cowrie_connector.record_login(username=username, password=password)
      auth_success = True
    except paramiko.AuthenticationException:
      auth_success = False
    except Exception as e:
      print(f"Cowrie auth check error: {e}")
      auth_success = False

    log_event.log_auth_event(self.client_addr, HOST, PORT, username, password, auth_success)
    return paramiko.AUTH_SUCCESSFUL if auth_success else paramiko.AUTH_FAILED

  def check_channel_request(self, kind, chanid):
    if kind == "session":
      return paramiko.OPEN_SUCCEEDED
    return paramiko.OPEN_FAILED_ADMINISTRATIVELY_PROHIBITED

  def check_channel_pty_request(self, channel, term, width, height, pixelwidth, pixelheight, modes):
    return True

  def check_channel_shell_request(self, channel):
    return True

  def check_channel_exec_request(self, channel, command):
    return True

def start_proxy():
  sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  sock.bind((HOST, PORT))
  sock.listen(100)
  print(f"SSH Proxy listening on {HOST}:{PORT}")

  while True:
    try:
      client, addr = sock.accept()
      print(f"Connection from {addr}")

      transport = paramiko.Transport(client)
      transport.add_server_key(HOST_KEY)
      server = SSHProxyServer(addr)

      try:
        transport.start_server(server=server)
      except paramiko.SSHException:
        print("SSH negotiation failed")
        continue
      except EOFError:
        print("Client closed connection during handshake (EOF)")
        continue
      except Exception as e:
        print(f"Unexpected error during SSH handshake: {e}")
        continue

      try:
        chan = transport.accept(20)
        if chan is None:
          print("No channel")
          transport.close()
          client.close()
          continue

        username = server.username
        password = server.password

        start_time = time.time()

        threading.Thread(
          target=handler.handle_session,
          args=(chan, username, password, addr, start_time, server.cowrie_connector),
          daemon=True
        ).start()

      except EOFError:
        print("Client closed connection after authentication (EOF)")

        try:
          transport.close()
        except:
          pass

        try:
          client.close()
        except:
          pass

      except Exception as e:
        print(f"Error during session handling: {e}")

        try:
          transport.close()
        except:
          pass

        try:
          client.close()
        except:
          pass

    except Exception as e:
      print(f"Error acceptiong connection: {e}")

if __name__ == "__main__":
  start_proxy()
