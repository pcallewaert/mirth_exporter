#! /bin/sh
set -e

# Download the correct version of Mirth Connect CLI and extract it
if [ -z "$MIRTH_VERSION" ]
then
      echo "\$MIRTH_VERSION is mandatory, build number should be attached! eg. 3.8.0.b2464"
fi
wget https://s3.amazonaws.com/downloads.mirthcorp.com/connect/$MIRTH_VERSION/mirthconnectcli-$MIRTH_VERSION-unix.tar.gz

tar xvfz mirthconnectcli-$MIRTH_VERSION-unix.tar.gz
# Update the config
if [ -z "$MIRTH_SERVER" ]
then
      echo "\$MIRTH_SERVER is empty, using default"
else
      echo "Using $MIRTH_SERVER to connect"
      sed -i "s#address=https://127.0.0.1:8443#address=$MIRTH_SERVER#g" /home/app/Mirth\ Connect\ CLI/conf/mirth-cli-config.properties
fi
if [ -z "$MIRTH_USERNAME" ]
then
      echo "\$MIRTH_USERNAME is empty, using default"
else
      echo "Using $MIRTH_USERNAME to connect"
      sed -i "s#user=admin#user=$MIRTH_USERNAME#g" /home/app/Mirth\ Connect\ CLI/conf/mirth-cli-config.properties
fi
if [ -z "$MIRTH_PASSWORD" ]
then
      echo "\$MIRTH_PASSWORD is empty, using default"
else
      echo "Using $MIRTH_PASSWORD to connect"
      sed -i "s#password=admin#password=$MIRTH_PASSWORD#g" /home/app/Mirth\ Connect\ CLI/conf/mirth-cli-config.properties
fi

# Start the exporter
/home/app/mirth_exporter -mccli.path "./Mirth Connect CLI/mccommand"