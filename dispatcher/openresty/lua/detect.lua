local http = require("resty.http")

local raw_uri = ngx.var.request_uri or ""
local uri     = raw_uri:lower()
local dec_uri = ngx.unescape_uri(uri)
local ua      = (ngx.var.http_user_agent or ""):lower()

local high_patterns = {
  "sqlmap", "python-requests", "python", "curl", "wget", "nmap", "masscan", "nikto",
  "../", "/etc/passwd", "c:\\windows\\system32", "/proc/self/environ",
  "or 1=1", "' or '1'='1", "\" or \"1\"=\"1", "union select", "sleep(", "benchmark(",
  "cmd.exe", "powershell"
}

local wordpress_patterns = {
  "wp-login.php", "xmlrpc.php", "wp-admin",
  "wp-content", "wp-includes", "wp-json", "wp-config.php",
  "wp-comments-post.php", "wp-cron.php", "wp-"
}

local function match_any_in(strs, patterns)
  for _, s in ipairs(strs) do
    if s then
      for _, p in ipairs(patterns) do
        if s:find(p, 1, true) then
          return true
        end
      end
    end
  end
  return false
end

local is_wp   = match_any_in({ uri, dec_uri }, wordpress_patterns)
local is_high = match_any_in({ uri, dec_uri, ua }, high_patterns)

local rules = {
  { target = "wordpot",  match = function() return is_wp end },
  { target = "h0neytr4p", match = function() return (not is_wp) and is_high end },
}

local function proxy(target)
  return ngx.exec("@" .. target)
end

for _, rule in ipairs(rules) do
  local ok = false
  local ok_call, res = pcall(rule.match)

  if ok_call and res then
    return proxy(rule.target)
  end
end
