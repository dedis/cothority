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

RUN apt-get update -y -o Acquire::Check-Valid-Until=false

RUN apt-get install -y --force-yes """

Closer = "RUN apt-get autoremove -y && apt-get clean -y"
