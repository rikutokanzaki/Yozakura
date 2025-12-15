import logging

logger = logging.getLogger(__name__)

class Authenticator:
  def __init__(self, user_file="./config/user.txt"):
    self.rules = []

    try:
      with open(user_file, "r") as f:
        for line in f:
          line = line.strip()
          if not line or ":" not in line or line.startswith("#"):
            continue
          user, passwd = line.split(":", 1)
          self.rules.append((user, passwd))
    except FileNotFoundError:
      logger.warning("User file '%s' not found.", user_file)
      self.rules = []

  def authenticate(self, username: str, password: str) -> bool:
    for rule_user, rule_pass in self.rules:
      if rule_user == username or rule_user == "*":
        if rule_pass == "*":
          return True
        elif rule_pass.startswith("!"):
          return password != rule_pass[1:]
        else:
          return password == rule_pass
    return False
