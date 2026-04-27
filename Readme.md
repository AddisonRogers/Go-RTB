# Go-RTB

A Go-based real-time bidding (RTB) system. This is a work in progress.

## Architecture
The short and the rough is that we have the 
- 'Client' that acts as the point of interface for creating campaigns, accounts, websites topping up etc.
- The 'Banker' is the service that handles the approval / limiter of the bid requests.
- The 'Exchange' is the service that handles the exchange of the bid requests. 
It takes a bid request, looks up the website and ads that are closest to the website in similarity, 
then sends a value request to a bidder with each of the top n ads.
- (currently working on this) The 'Bidder' is the service that handles the bid requests. It takes a value request and sends it *somehow* to a regression model with all the info as features to predict a value of the request.
- The 'Worker' is a background service that handles a few different things. For one it handles the banker's rate limiting (refer to [this technical challenge](#throughput-computation-on-expiry) for more info).

## Routes

All services have a /health route that returns a 200 if the service is up.

### Client (API for client)
- GET /accounts
- GET /accounts/{id}
- POST /accounts/{id}
- PUT /accounts/{id}
- DELETE /accounts/{id}

- GET /campaigns
- GET /campaigns/{id}
- PATCH /campaigns/{id}
    - This likely wont be implemented as it's unneeded for now but REST best practice afaik is to not have a /topup
- POST /campaigns/{id}/topup
- PUT /campaigns/{id}
- DELETE /campaigns/{id}

### Banker
- POST /campaigns/{id}/authorize
- POST /campaigns/{id}/clear

### Bidder
- POST /bid

### Exchange
- POST /exchange

## TODO
- [ ] Add auth and such to the client available api
- [ ] Handle retries (from api to redis) and such in a graceful manner
- [ ] Add a proper logger
- [ ] Add more meaningful error messages

## Random extension ideas
- [ ] "Rat" system to add more data for ads or websites to the embedding to have better matches.
- [ ] Use some classification system to classify ads or websites in order to not have linear regression models on a per website basis.

## Technical challenges / decisions

### Throughput computation on expiry
So the issue was that I need to approve/deny the bidrequest in the banker based off of the computed amount of spend / 10 minutes.
This is obviously an issue as I will need to either compute the property each time a bid is sent or calculate it different time.
The way I decided to do it is obviously calculate + change when there has been a clear/approve operation but how do I handle expiries
I considered using a goroutine to handle things in the background but ultimately this went against my microservice plan.
So I instead wanted to use a queue + a worker of some kind. The next question was how do I make it so that I dont have the worker polling
constantly the same db and having a poor scale up. So I looked up about redis and found keyspace notifications which solved a big part of this.
BUT redis keyspace notifications are not reliable. So instead I opt for a sorted set which uses the timestamp as the score.

### How to do "best" ad and aligning closest with interests
For each website and add we will have a vector db to calculate the closest in similarity to the website. 
This will produce an ordered list of ads that are closest to the website in similarity.

This then with all the other metrics we can possibly get will be put into a linear regression model to get final performing scores. 
Then we can multiply the score by bid amount and then sort the list by that.
