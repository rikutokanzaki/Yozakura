class AttackDetector:
  def __init__(self):
    self.failed_attempts = 0

  def check(self, username, password, command):
    if username == "root" and password == "1234":
      return True
    if "wget" in command or "curl" in command:
      return True
    return False
