FROM openresty/openresty:1.21.4.1-0-jammy

RUN /usr/local/openresty/luajit/bin/luarocks install lua-resty-http

COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf
COPY conf.d/ /etc/nginx/conf.d/
COPY lua/ /etc/nginx/lua/

CMD ["openresty", "-g", "daemon off;"]
