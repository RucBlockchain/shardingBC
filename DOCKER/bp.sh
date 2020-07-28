PROPATH=$( cat package.js | sed 's/,/\n/g' | grep "propath" | sed 's/:/\n/g' | sed '1d' | sed 's/}//g' | sed 's/"//g')

rm tendermint

cd $PROPATH

make build

cd ./DOCKER

cp ../build/tendermint ./

sudo docker build -t "tendermint:latest" -t "tendermint:v3.0" .


