# Challenges

## Throughput computation on expiry
So the issue was that I need to approve/deny the bidrequest in the banker based off of the computed amount of spend / 10 minutes.
This is obviously an issue as I will need to either compute the property each time a bid is sent or calculate it different time.
The way I decided to do it is obviously calculate + change when there has been a clear/approve operation but how do I handle expiries
I considered using a goroutine to handle things in the background but ultimately this went against my microservice plan.
So I instead wanted to use a queue + a worker of some kind. The next question was how do I make it so that I dont have the worker polling 
constantly the same db and having a poor scale up. So I looked up about redis and found keyspace notifications which solved a big part of this.
BUT redis keyspace notifications are not reliable. So instead I opt for a sorted set which uses the timestamp as the score.

## How to do "best" ad and aligning closest with interests
- [ ] For each website and add we will have a vector db to calculate the closest in similarity to the website. This will produce an ordered list of ads that are closest to the website in similarity.
- [ ] This then with all the other metrics we can possibly get will be put into a linear regression model to get final performing scores. Then we can multiply the score by bid amount and then sort the list by that.

## Random ideas
- [ ] Rat system to add more data for ads or websites to the embedding to have better matches.
- [ ] Use some classification system to classify ads or websites in order to not have linear regression models on a per website basis.


# TODO but not really
- [ ] Add auth and such to the client available api
- [ ] Handle retries (from api to redis) and such in a graceful manner
- [ ] Add a proper logger
- [ ] Add more meaningful error messages