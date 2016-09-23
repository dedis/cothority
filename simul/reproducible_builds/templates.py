Header1 = """FROM debian:jessie

# Add unstable repos with newest package versions
RUN echo 'deb http://httpredir.debian.org/debian stretch main' >> /etc/apt/sources.list \\
 && echo 'deb-src http://httpredir.debian.org/debian stretch main' >> /etc/apt/sources.list \\
 && echo 'deb http://reproducible.alioth.debian.org/debian/ ./' | tee -a /etc/apt/sources.list \\
 && echo 'deb-src http://reproducible.alioth.debian.org/debian/ ./' | tee -a /etc/apt/sources.list \\
 """

Header2 = """
# Add a public of repository for reproducible versions of packages
RUN apt-key adv --keyserver http://reproducible.alioth.debian.org/reproducible.asc --recv-keys 49B6574736D0B637CC3701EA5DB7CA67EA59A31F

RUN cp /etc/hosts /tmp/hosts
RUN mkdir -p -- /lib-override && cp /lib/x86_64-linux-gnu/libnss_files.so.2 /lib-override
RUN perl -pi -e 's:/etc/hosts:/tmp/hosts:g' /lib-override/libnss_files.so.2
ENV LD_LIBRARY_PATH /lib-override
RUN echo 193.62.202.30 snapshot.debian.org >> /tmp/hosts

RUN echo 'Acquire::Check-Valid-Until "false";' >> /etc/apt/apt.conf
# RUN echo 'Acquire::proxy::http "http://icsil1-conode1.epfl.ch:3142";' >> /etc/apt/apt.conf
RUN apt-get update -y

RUN apt-get install -y --force-yes """

Closer = "RUN apt-get autoremove -y && apt-get clean -y"
