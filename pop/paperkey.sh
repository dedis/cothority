#!/usr/bin/env bash

IMG_PUB=1000020100000190000001900EE32AC40F291B4D.png
IMG_PRIV=1000020100000190000001908A01F6F07EE83777.png
OUT=$(pwd)

for i in $( seq -f "%02g" 50 ); do
	OFILE=$OUT/key$i.odg
	rm -f $OFILE
	KP=$( mktemp )
	./pop client create > $KP
	PRIV=$( grep Private $KP | sed -e "s/.* //")
	PUB=$( grep Public $KP | sed -e "s/.* //")
	TMP=$( mktemp -d )
	unzip -q paperkey.odg -d $TMP
	perl -pi -e "s-Public_base64-$PUB-" $TMP/content.xml
	perl -pi -e "s-Private_base64-$PRIV-" $TMP/content.xml
	qrencode -o $TMP/Pictures/$IMG_PUB ed25519pub:$PUB
	qrencode -o $TMP/Pictures/$IMG_PRIV ed25519priv:$PRIV
	( cd $TMP; zip -qr $OFILE . )
	rm -rf $KP $TMP
	/Applications/LibreOffice.app/Contents/MacOS/soffice --headless --convert-to pdf $OFILE
done
