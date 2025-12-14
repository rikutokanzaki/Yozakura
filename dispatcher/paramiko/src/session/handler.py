from session import set_prompt
from reader import line_reader
from utils import set_motd, ansi_sequences, log_event, resource_manager
import logging
import os
import time

logger = logging.getLogger(__name__)

def _build_dir_cmd(cwd: str) -> str:
  if not cwd or cwd == "~":
    return ""
  return f"cd {cwd}"

def handle_session(chan, username, password, addr, start_time, cowrie_connector):
  history = []
  dir_cmd = ""

  hostname = str(os.getenv('HOST_NAME'))[:9]
  cwd = "~"

  prompt_manager = set_prompt.PromptManager()
  prompt = prompt_manager.get_prompt(username, hostname, cwd)
  reader = line_reader.LineReader(chan, username, password, prompt, history, cowrie_connector=cowrie_connector, cwd=cwd)

  try:
    cowrie_connector.flush_buffer(timeout=1.0)
  except Exception:
    pass

  motd_lines = set_motd.get_motd_lines(hostname)
  chan.send(b"\r\n")
  for line in motd_lines:
    sent_line = line.rstrip() + "\r\n"
    chan.send(sent_line.encode("utf-8"))
    time.sleep(0.005)

  try:
    while True:
      cmd = reader.read()

      if not cmd:
        continue

      try:
        src_ip, src_port = chan.getpeername()
      except:
        src_ip, src_port = "unknown", 0

      log_event.log_command_event(src_ip, src_port, username, cmd, cwd)

      if cmd.lower() in ["exit", "quit", "exit;", "quit;"]:
        break

      dir_cmd = _build_dir_cmd(cwd)

      try:
        output, cwd = cowrie_connector.execute_command(cmd, username, password, dir_cmd)
      except Exception:
        logger.exception("Cowrie connection lost during command execution")
        chan.send(b"Connection to backend lost. Session terminated.\r\n")
        break

      dir_cmd = _build_dir_cmd(cwd)
      prompt = prompt_manager.get_prompt(username, hostname, cwd)
      reader.update_prompt(prompt)

      clean_output = ansi_sequences.strip_ansi_sequences(output)
      chan.send(clean_output.encode("utf-8"))

  except EOFError:
    logger.info("Client closed connection (EOF)")

  except Exception:
    logger.exception("Error handling session")

  finally:
    try:
      src_ip, src_port = addr[0], addr[1]
    except Exception:
      src_ip, src_port = "unknown", 0

    duration = time.time() - start_time
    log_event.log_session_close(
      src_ip=src_ip,
      src_port=src_port,
      username=username,
      duration=duration,
      message="Session closed"
    )

    try:
      reader.cleanup_terminal()
    except Exception:
      logger.exception("Failed to cleanup terminal")

    resource_manager.close_channel(chan)
