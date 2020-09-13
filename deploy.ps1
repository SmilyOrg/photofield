docker-compose build

echo Sending
docker save photofield_photofield -o \\Denkarium\docker\photofield-image.tar

echo Loading
ssh -t admin@denkarium "bash --login -c 'cd /volume1/docker/; sudo docker load -i photofield-image.tar; sudo docker-compose up -d photofield'"