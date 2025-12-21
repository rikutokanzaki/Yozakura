from connector import connect_server
from utils import ansi_sequences, extract_chars
import logging

logger = logging.getLogger(__name__)

class LineReader:
  def __init__(self, chan, username, password, prompt="", history=[], cowrie_connector=None, cwd="~"):
    self.chan = chan
    self.username = username
    self.password = password
    self.prompt = prompt
    self.buffer = []
    self.cursor_pos = 0
    self.escape_seq = b""
    self.prev_rendered_len = 0
    self.history = history
    self.history_index = -1
    self.max_history_length = 1000
    self.cwd = cwd
    self.cowrie_connector = cowrie_connector

  def update_prompt(self, new_prompt):
    self.prompt = new_prompt

  def update_cwd(self, new_cwd):
    self.cwd = new_cwd

  def send_prompt(self):
    self.chan.send(self.prompt.encode("utf-8"))

  def redraw_buffer(self):
    self.chan.send(b"\r")
    self.send_prompt()

    rendered = (b"".join(self.buffer))
    self.chan.send(rendered)

    if self.prev_rendered_len > len(self.buffer):
      diff = self.prev_rendered_len - len(self.buffer)
      self.chan.send(b" " * diff)
      self.chan.send(f"\x1b[{diff}D".encode())

    back = len(rendered) - self.cursor_pos
    if back > 0:
      self.chan.send(f"\x1b[{back}D".encode())

    self.prev_rendered_len = len(rendered)

  def set_buffer_from_history(self):
    if 0 <= self.history_index < len(self.history):
      self.buffer = [c.encode("utf-8") for c in self.history[self.history_index]]
      self.cursor_pos = len(self.buffer)
      self.redraw_buffer()
      self.prev_rendered_len = len(self.buffer)
    else:
      logger.debug("Invalid history index in LineReader.set_buffer_from_history")

  def handle_tab_completion(self):
    full_input = b"".join(self.buffer).decode("utf-8", errors="ignore")
    tokens = full_input.strip().split()
    if not tokens:
      return

    last_token = tokens[-1]

    command_with_tab = full_input + "\t"

    connector = self.cowrie_connector or connect_server.SSHConnector(host="cowrie", port=2222)
    cwd = self.cwd or "~"

    command, output_chars = connector.execute_with_tab(
      cwd,
      command_with_tab,
      self.username,
      self.password
    )

    output_chars_clean = ansi_sequences.strip_ansi_sequences(output_chars)
    completed_command = extract_chars.get_completion_diff(command.strip(), output_chars_clean.strip())
    completion_diff = completed_command[len(command.strip()):]

    if completion_diff:
      buffer_str = b''.join(self.buffer).decode("utf-8", errors="ignore")
      token_start = buffer_str.rfind(last_token)

      if token_start != -1:
        insertion_index = token_start + len(last_token)
        for ch in completion_diff:
          self.buffer.insert(insertion_index, ch.encode('utf-8'))
          insertion_index += 1
        self.cursor_pos = insertion_index

        self.redraw_buffer()

  def handle_escape_sequence(self):
    try:
      seq = self.chan.recv(2)
    except Exception as e:
      logger.exception("Failed to read escape sequence")
      return

    # UP
    if seq == b"[A":
      if self.history:
        if self.history_index == -1:
          self.history_index = len(self.history) - 1
        else:
          if self.history_index > 0:
            self.history_index -= 1
        self.set_buffer_from_history()

    # DOWN
    elif seq == b"[B":
      if self.history and self.history_index < len(self.history) - 1:
        self.history_index += 1
        self.set_buffer_from_history()

    # RIGHT
    elif seq == b"[C":
      if self.cursor_pos < len(self.buffer):
        self.cursor_pos += 1
        self.chan.send(b"\x1b[C")

    # LEFT
    elif seq == b"[D":
      if self.cursor_pos > 0:
        self.cursor_pos -= 1
        self.chan.send(b"\x1b[D")

    # DELETE
    elif seq == b"[3":
      try:
        t = self.chan.recv(1)
        if t == b"~" and self.cursor_pos < len(self.buffer):
          del self.buffer[self.cursor_pos]
          if self.cursor_pos == len(self.buffer):
            self.chan.send(b" \b")
          else:
            remainder = b"".join(self.buffer[self.cursor_pos:]) + b" "
            self.chan.send(remainder)
            self.chan.send(f"\x1b[{len(remainder)}D".encode())
      except Exception as e:
        logger.exception("Failed to read DELETE escape sequence")
        return

  def read(self):
    self.buffer = []
    self.cursor_pos = 0
    self.history_index = -1
    self.chan.send(b"\r\x1b[2K")
    self.send_prompt()

    while True:
      try:
        data = self.chan.recv(1)
        if not data:
          break

        if data == b"\x1b":
          self.handle_escape_sequence()
          continue

        # ENTER
        if data in (b"\n", b"\r"):
          self.chan.send(b"\r\n")
          line = b"".join(self.buffer).decode("utf-8", errors="ignore")
          if line:
            self.history.append(line)
            if len(self.history) > self.max_history_length:
              self.history.pop(0)
          return line

        # BACKSPACE
        if data in (b"\x7f", b"\x08"):
          if self.cursor_pos > 0:
            del self.buffer[self.cursor_pos - 1]
            self.cursor_pos -= 1
            if self.cursor_pos == len(self.buffer):
              self.chan.send(b"\b \b")
            else:
              remainder = b"".join(self.buffer[self.cursor_pos:]) + b" "
              self.chan.send(b"\b" + remainder)
              self.chan.send(f"\x1b[{len(remainder)}D".encode())
          continue

        # TAB
        if data == b"\t":
          self.handle_tab_completion()
          continue

        self.buffer.insert(self.cursor_pos, data)
        self.cursor_pos += 1

        if self.cursor_pos == len(self.buffer):
          self.chan.send(data)
        else:
          remainder = b"".join(self.buffer[self.cursor_pos - 1:])
          self.chan.send(remainder)
          self.chan.send(f"\x1b[{len(remainder) - 1}D".encode())

      except Exception:
        logger.exception("Error while reading from channel")
        break

    return ""

  def cleanup_terminal(self):
    self.chan.send(b"\x1b[0m")
