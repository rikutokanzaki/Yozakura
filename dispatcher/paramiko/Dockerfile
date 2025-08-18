FROM python:3.10-slim

RUN apt-get update && \
  apt-get install -y --no-install-recommends openssh-client && \
  rm -rf /var/lib/apt/lists/*

RUN mkdir -p /certs && \
  ssh-keygen -t rsa -b 2048 -f /certs/ssh_host_rsa_key -N ""

COPY . .

WORKDIR /

RUN pip install -r requirements.txt
ENV PYTHONUNBUFFERED=1

CMD ["python", "./src/main.py"]
