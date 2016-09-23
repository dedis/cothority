FROM debian:jessie

RUN echo 'deb http://reproducible.alioth.debian.org/debian/ ./' > /etc/apt/sources.list \
 && echo 'deb-src http://reproducible.alioth.debian.org/debian/ ./' >> /etc/apt/sources.list

# Add a public of repository for reproducible versions of packages
RUN apt-key adv --keyserver http://reproducible.alioth.debian.org/reproducible.asc --recv-keys 49B6574736D0B637CC3701EA5DB7CA67EA59A31F; \
  apt-get update; \
  apt-get dist-upgrade -y
