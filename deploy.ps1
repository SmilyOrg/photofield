docker-compose build

echo Sending
docker save photofield_photofield -o \\Denkarium\docker\photofield-image.tar

echo Loading
ssh -t admin@denkarium "bash --login -c 'sudo docker load -i /volume1/docker/photofield-image.tar; sudo docker restart photofield'"
