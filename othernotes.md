# Challenges

## Throughput computation on expiry
So the issue was that I need to approve/deny the bidrequest in the banker based off of the computed amount of spend / 10 minutes.
This is obviously an issue as I will need to either compute the property each time a bid is sent or calculate it different time.
The way I decided to do it is obviously calculate + change when there has been a clear/approve operation but how do I handle expiries
I considered using a goroutine to handle things in the background but ultimately this went against my microservice plan.
So I instead wanted to use a queue + a worker of some kind. The next question was how do I make it so that I dont have the worker polling 
constantly the same db and having a poor scale up. So I looked up about redis and found keyspace notifications which solved a big part of this.
BUT redis keyspace notifications are not reliable. So instead I opt for a sorted set which uses the timestamp as the score.