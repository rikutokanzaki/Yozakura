from utils import ansi_sequences, resource_manager
import logging
import paramiko
import re
import time

logger = logging.getLogger(__name__)

class SSHConnector:
  def __init__(self, host: str, port: int = 22):
    self.host = host
    self.port = port
    self.shell = None

  def record_login(self, username: str, password: str):
    client = None
    shell = None
    transport = None

    try:
      client = paramiko.SSHClient()
      client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
      client.connect(self.host, port=self.port, username=username, password=password, timeout=10)
      transport = client.get_transport()

      try:
        shell = client.invoke_shell()
        shell.settimeout(5)
      except Exception:
        logger.debug("Shell not available during heralding login record")

    except Exception:
      logger.exception("Login recording error")
      raise

    finally:
      resource_manager.close_ssh_connection(client=client, shell=shell, transport=transport)

  def flush_buffer(self, timeout: float = 0.2):
    if not self.shell:
      return
    end = time.time() + timeout

    try:
      while time.time() < end:
        if self.shell.recv_ready():
          _ = self.shell.recv(4096)

        else:
          time.sleep(0.02)

    except Exception:
      pass

  def execute_command(self, command: str, username: str, password: str, dir_cmd=None):
    client = None
    shell = None
    transport = None

    try:
      client = paramiko.SSHClient()
      client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
      client.connect(self.host, port=self.port, username=username, password=password, timeout=10)
      transport = client.get_transport()

      shell = client.invoke_shell()
      shell.settimeout(5)

      self._wait_for_prompt(shell)

      if dir_cmd:
        shell.send(dir_cmd + "\n")
        self._wait_for_prompt(shell)

      shell.send(command + "\n")
      output, cwd = self._receive_until_prompt(shell, command)

      return output, cwd

    except Exception:
      logger.exception("Error in execute_command")
      raise

    finally:
      resource_manager.close_ssh_connection(client=client, shell=shell, transport=transport)


  def execute_with_tab(self, cwd, command: str, username: str, password: str):
    client = None
    shell = None
    transport = None

    try:
      client = paramiko.SSHClient()
      client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
      client.connect(self.host, port=self.port, username=username, password=password, timeout=10)
      transport = client.get_transport()

      shell = client.invoke_shell()
      shell.settimeout(5)

      self._wait_for_prompt(shell)

      shell.send(f"cd {cwd}\n")
      self._wait_for_prompt(shell)

      raw_command = command.replace("\t", "")
      shell.send(raw_command + "\t")
      time.sleep(0.2)

      output = b""
      start_time = time.time()
      timeout = 1

      while True:
        try:
          if shell.recv_ready():
            chunk = shell.recv(1024)
            output += chunk
            decoded = output.decode("utf-8", errors="ignore")
            cleaned = ansi_sequences.strip_ansi_sequences(decoded)

            if raw_command in cleaned:
              index = cleaned.rfind(raw_command)
              if index != -1 and len(cleaned) > index + len(raw_command):
                break

          if time.time() - start_time > timeout:
            break
          time.sleep(0.05)

        except Exception:
          logger.exception("Error while receiving TAB completion output")
          break

      output_chars = output.decode("utf-8", errors="ignore")

      return command, output_chars

    except Exception:
      logger.exception("Error in execute_with_tab")
      return "", ""

    finally:
      resource_manager.close_ssh_connection(client=client, shell=shell, transport=transport)

  def _wait_for_prompt(self, shell):
    try:
      while True:
        data = shell.recv(1024)

        if not data:
          break

        if b"$ " in data or b"# " in data:
          break

    except Exception:
      logger.exception("Error in _wait_for_prompt")
      raise

  def _receive_until_prompt(self, shell, sent_cmd: str = "") -> tuple[str, str]:
    output = b""
    prompt_line = b""

    try:
      while True:
        data = shell.recv(1024)
        if not data:
          break
        output += data
        if b"$ " in data or b"# " in data:
          prompt_line = data
          break
    except Exception:
      logger.exception("Error in _receive_until_prompt")
      raise

    lines = output.split(b"\n")
    cleaned_lines = []

    for i, line in enumerate(lines):
      if sent_cmd.encode("utf-8") in line.strip():
        continue
      if i == len(lines) - 1:
        try:
          line_str = line.decode("utf-8", errors="ignore")
          prompt_str = prompt_line.decode("utf-8", errors="ignore").strip()
          cleaned_line_str = ansi_sequences.remove_prompt(line_str)
          cleaned_lines.append(cleaned_line_str.encode("utf-8"))
        except Exception:
          cleaned_lines.append(line)
      else:
        cleaned_lines.append(line)

    output_lines = b"\n".join(cleaned_lines).decode("utf-8", errors="ignore")

    cwd = "~"
    match = re.search(r"@[^:]+:(.*?)[\$#] ?", prompt_str)
    if match:
      cwd = match.group(1).strip()

    return output_lines, cwd
