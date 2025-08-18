def get_completion_diff(original: str, completed: str) -> str:
  if completed.startswith(original):
    return completed[len(original):]
  return ""
