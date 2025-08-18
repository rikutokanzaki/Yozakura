import json
import datetime

def log_auth_event(addr, dest_ip, dest_port, username, password, success):
  log = {
    "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "type": "Paramiko",
    "eventid": "paramiko.login.attempt",
    "src_ip": addr[0],
    "src_port": addr[1],
    "dest_ip": dest_ip,
    "dest_port": dest_port,
    "username": username,
    "password": password,
    "protocol": "ssh",
    "success": success
  }

  with open("/var/log/paramiko/paramiko.log", "a") as f:
    f.write(json.dumps(log) + "\n")

def log_command_event(src_ip, src_port, username, command, cwd):
  log = {
    "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "type": "Paramiko",
    "eventid": "paramiko.command.input",
    "src_ip": src_ip,
    "src_port": src_port,
    "username": username,
    "command": command,
    "cwd": cwd,
    "protocol": "ssh"
  }
  with open("/var/log/paramiko/paramiko.log", "a") as f:
    f.write(json.dumps(log) + "\n")

def log_session_close(src_ip, src_port, username, duration, message):
  log = {
    "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "type": "Paramiko",
    "eventid": "paramiko.session.close",
    "src_ip": src_ip,
    "src_port": src_port,
    "username": username,
    "duration": f"{round(duration, 2)}s",
    "message": message,
    "protocol": "ssh"
  }
  with open("/var/log/paramiko/paramiko.log", "a") as f:
    f.write(json.dumps(log) + "\n")
