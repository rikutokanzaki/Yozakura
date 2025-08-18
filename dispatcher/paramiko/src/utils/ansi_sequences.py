import re

ANSI_ESCAPE_RE = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')

def strip_ansi_sequences(text):
  return ANSI_ESCAPE_RE.sub("", text)

def remove_prompt(text: str) -> str:
  matches = list(re.finditer(r'\x1b\[4.', text))
  if not matches:
    return text

  last_match = matches[-1]
  cut_index = last_match.start()
  return text[:cut_index].rstrip()

def extract_command(text: str) -> str:
  cleaned = strip_ansi_sequences(text)

  match = re.search(r'[$#]\s*(.*)', cleaned)
  if match:
    return match.group(1).strip()

  return ""
