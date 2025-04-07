# Blockchain Indexer Pro

## Overview

Blockchain Indexer Pro is a powerful, flexible solution for indexing and storing blockchain data directly into your PostgreSQL database. Built 

## Features

- 🔍 Multiple Indexer Types:
  - NFT Bids Tracking
  - NFT Prices Tracking
  - Token Borrowing Data
  - Token Prices Tracking

- 🔒 Secure Authentication
  - JWT-based user authentication
  - Password hashing with Argon2

- 🗃️ Database Management
  - Store multiple database credentials
  - Create indexers connected to your own databases

- 🌐 Webhook Integration
  - Uses Helius API for real-time blockchain data streaming
  - Supports custom webhook configurations

- 📊 Comprehensive Logging
  - Detailed indexing logs
  - Error tracking and status monitoring

## Prerequisites

### Backend
- Go: 1.22+
- PostgreSQL 17
- Helius API Key

### Frontend
- Node.js: 22.14.0
- React 18+
- Tailwind CSS

## Technology Stack

### Backend
- Language: Go
- Web Framework: Gin
- Database: PostgreSQL (pgx)
- ORM/Query Generator: sqlc
- Authentication: JWT, Argon2
- Logging: zerolog

### Frontend
- Language: TypeScript
- Framework: React
- State Management: Context API
- UI Library: Shadcn/UI
- Styling: Tailwind CSS
- HTTP Client: Axios

### Infrastructure
- Database Migrations: golang-migrate
- Configuration: Viper
- Dependency Management: Go Modules

## Getting Started

### Backend Setup

1. Clone the repository
```bash
git clone https://github.com/rishavmehra/indexer.git
cd indexer
```

2. Install dependencies
```bash
go mod tidy
```

3. Create a `.env` file with the following configurations:
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_username
DB_PASSWORD=your_password
DB_NAME=indexer_db
DB_SSL_MODE=disable

JWT_SECRET=your_jwt_secret
JWT_EXPIRES_IN=24h

HELIUS_API_KEY=your_helius_api_key
HELIUS_WEBHOOK_BASE_URL=http://localhost:8080 # use ngrok to test locally
```

4. Migrate the database
```
Install Migration CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

Create Migrations
migrate -path internal/db/migrations -database "postgres://myuser3:mypassword3@localhost:5430/mydatabase3?sslmode=disable" up

Down Migrations
migrate -path internal/db/migrations -database "postgres://myuser3:mypassword3@localhost:5430/mydatabase3?sslmode=disable" down

```


5. Start the server
```bash
go run cmd/server/main.go
```

### Frontend Setup

1. Navigate to the frontend directory
```bash
cd web
```

2. Install dependencies
```bash
npm install
```

3. Create a `.env` file
```
VITE_BACKEND_API_BASE_URL=http://localhost:8080/api/v1
```

4. Start the development server
```bash
npm run dev
```

## Core Indexer Types

### NFT Bids Indexer
- Track bids for specific NFT collections
- Filter by marketplaces
- Store bid details in your database

### NFT Prices Indexer
- Monitor price changes for NFT collections
- Track listings, sales, and cancellations
- Filter by specific marketplaces

### Token Borrow Indexer
- Capture token borrowing activities
- Track lending platform data
- Record borrow and repay events

### Token Prices Indexer
- Real-time token price tracking
- Multiple platform support
- Capture price, volume, and market data

## Security Features

- Argon2 password hashing
- JWT authentication
- Role-based access control
- Secure database credential management

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

Distributed under the MIT License. See `LICENSE` for more information.

## Contact

[rishavmehra61@gmail.com](mailto:rishavmehra61@gmail.com)

Project Link: [https://github.com/rishavmehra/indexer](https://github.com/rishavmehra/indexer)

## Acknowledgements

Made with ❤️ by [Rishav Mehra](https://x.com/rishavmehraa)