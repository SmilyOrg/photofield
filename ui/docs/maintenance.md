# Maintenance

Over time the cache database can grow in size due to version upgrades.
To shrink the database to its minimum size, you can _vacuum_ it. Multiple vacuums in a row have no effect as the vacuum itself rewrites the database from
the ground up.

While the vacuum is in progress, it will take twice the database size and may
take several minutes if you have lots of photos and a low-power system.

As an example it took around 5 minutes to vacuum a 260 MiB database containing around 500k photos on a DS418play. The size after vacuuming was 61 MiB as all the
leftover data from database upgrades was cleaned up.

## How to Vacuum

1. Shut down the server
2. Run the following command
    ::: code-group
    ```sh [CLI]
    ./photofield -vacuum
    ```

    ```sh [Docker]
    docker exec -it photofield ./photofield -vacuum
    ```
    :::
3. Restart the server