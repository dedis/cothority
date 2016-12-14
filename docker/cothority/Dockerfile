# Cothority-package in a docker-file
#
# VERSION               0.1

FROM debian
MAINTAINER Linus Gasser <linus.gasser@epfl.ch>

RUN echo "deb http://ftp.debian.org/debian jessie-backports main" >> /etc/apt/sources.list
RUN apt-get update && apt-get install -y \
	openssh-server \
	golang-1.6 \
	psmisc \
	inetutils-ping \
	git \
	vim \
	sudo
RUN adduser --gecos "" --ingroup sudo --disabled-password dedis; \
	perl -pi -e "s/%sudo.*/%sudo ALL=NOPASSWD: ALL/" /etc/sudoers
USER dedis
WORKDIR /home/dedis
RUN echo 'export GOPATH=/home/dedis/go' >> ~/.profile; \
   	echo 'export PATH=$PATH:/usr/lib/go-1.6/bin:~/bin:$GOPATH/bin' >> ~/.profile; \
   	echo 'export GOPATH=/home/dedis/go' >> ~/.bashrc; \
    echo 'export PATH=$PATH:/usr/lib/go-1.6/bin:~/bin:$GOPATH/bin' >> ~/.bashrc; \
	. ~/.profile;  \
	mkdir go; \
	go get -v github.com/dedis/cothority

EXPOSE 2000

