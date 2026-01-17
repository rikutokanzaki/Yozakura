FROM openresty/openresty:1.21.4.1-0-jammy

RUN apt-get update && \
  apt-get install -y git && \
  cd /tmp && \
  git clone https://github.com/ledgetech/lua-resty-http.git && \
  cd lua-resty-http && \
  /usr/local/openresty/luajit/bin/luarocks make && \
  cd / && \
  rm -rf /tmp/lua-resty-http && \
  apt-get remove -y git && \
  apt-get autoremove -y && \
  rm -rf /var/lib/apt/lists/*

COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf
COPY conf.d/ /etc/nginx/conf.d/
COPY lua/ /etc/nginx/lua/

CMD ["openresty", "-g", "daemon off;"]
