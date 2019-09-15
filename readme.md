# Kafka Consumer and Websocket server

1. Consumes Lat/Lon messages representing drone positions from a Kafka topic. 

2. Provides a websocket server.

3. Broadcasts the Kafka messages for all websocket clients.

### Issues:

1. At the moment it seems like the updates come in batches every 3-5 seconds. Reducing this would be better.

2. Not sure how to horizontally scale. Single topic with a single partition is consumed by a single process. Only method of scaling right now seems to be by adding more consumers in separate Kafka groups.
    * More consumers: ignored by Kafka since there's only one partition.
    * More partitions: Client connected to consumer-A won't see events arriving at consumer-B.
    * More consumers, one per new group: Consume all messages in tandem.

### Todo:

1. Come up with a better way to scale horizontally.
2. Let clients tell the server what messages they want to receive.
    * By area
    * By Droneport ID.
    * Etc
3. Use similar code to re-insert geofencing events into separate topics.