# DB schema and transactions

**Request ingestion**

In the store, before row insertion into requests, ensure that, with the upcoming insertion,
we haven't reached and are not reaching the bin's total 
- requests limit 
- stored body size limit
