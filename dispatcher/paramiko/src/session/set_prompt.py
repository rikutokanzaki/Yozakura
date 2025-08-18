class PromptManager:
  def __init__(self, cowrie_host="cowrie", cowrie_port=2222):
    self.cowrie_host = cowrie_host
    self.cowrie_port = cowrie_port

  def get_prompt(self, username, hostname, cwd="~"):
    return f"{username}@{hostname}:{cwd}$ "
