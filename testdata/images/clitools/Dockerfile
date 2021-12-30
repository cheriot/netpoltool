FROM alpine:3.14
RUN apk add --no-cache curl curl-doc bind-tools busybox-extras netcat-openbsd
RUN apk add --no-cache python3 py3-pip

RUN pip install flask
ENV FLASK_APP=server.py
ENV FLASK_ENV=production
RUN mkdir /opt/podinfo
WORKDIR /opt/podinfo
ADD server.py /opt/podinfo/server.py
EXPOSE 3000

CMD ["flask", "run", "-p", "3000", "--host", "0.0.0.0"]
