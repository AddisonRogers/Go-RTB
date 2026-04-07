# Routes

all have a /health

## Client (API for client)
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

## Banker
- POST /campaigns/{id}/authorize
- POST /campaigns/{id}/clear

## Bidder 
- POST /bid

## Exchange
- POST /exchange