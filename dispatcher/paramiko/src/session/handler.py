from session import set_prompt
from reader import line_reader
from utils import set_motd
from utils import ansi_sequences, log_event
import os
import time

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

      output, cwd = cowrie_connector.execute_command(cmd, username, password, dir_cmd or "")
      if cwd != "~":
        dir_cmd = f"cd {cwd}"
      else:
        dir_cmd = ""

      reader.update_cwd(cwd)

      prompt = prompt_manager.get_prompt(username, hostname, cwd)
      reader.update_prompt(prompt)
      clean_output = ansi_sequences.strip_ansi_sequences(output)
      chan.send(clean_output.encode("utf-8"))

  except Exception as e:
    print(f"Error handling session: {e}")

  finally:
    duration = time.time() - start_time
    log_event.log_session_close(
      src_ip=addr[0],
      src_port=addr[1],
      username=username,
      duration=duration,
      message="Session closed"
    )
    reader.cleanup_terminal()
    chan.close()
