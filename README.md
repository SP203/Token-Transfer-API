"# BTP Token Transfer API"

**Date:** 2025-10-23  
**Language:** Go  
**Database:** PostgreSQL  
**API Type:** GraphQL  

Description

This project implements a simple **GraphQL API** for transferring BTP tokens between wallets.  
Initially, there is one wallet holding `1,000,000 BTP` tokens.  
The API allows transferring tokens between wallets, ensuring that balances never go negative and that concurrent transfers are handled safely (race conditions are avoided).

How to Run the Application

Clone the Repository
git clone https://github.com/SP203/Token-Transfer-API.git
cd Token-Transfer-API

Make sure you have Docker installed, then run:

docker compose up -d
go run ./cmd/server/main.go

After starting the container, open your browser and go to:

http://localhost:8080/query

You’ll see the GraphQL Playground, where you can test your mutations.

mutation {
  transfer(
    from_address: "0x0000000000000000000000000000000000000000",
    to_address: "0xAlice",
    amount: 100
  )
}

#Insufficient balance

mutation {
  transfer(
    from_address: "0x0000000000000000000000000000000000000000",
    to_address: "0xBob",
    amount: 10000000
  )
}

This project includes end-to-end tests that connect to a real PostgreSQL instance and verify:
Successful transfers
Handling of insufficient balances
Race condition scenarios (multiple transfers at the same time)

To run the tests:

go test ./e2e -v

Author: Stanisław Piechowicz
