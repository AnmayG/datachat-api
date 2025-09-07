# DataChat API - Social Messenger Backend

A Go-based API backend with Stream Chat integration, user authentication, and **real-time handshake pub/sub system**.

## 🚀 New Feature: Handshake Pub/Sub System

This API now includes a simple, lightweight pub/sub system for broadcasting handshake events between users in real-time.

### Key Features

- ✅ **Zero Authentication**: Takes UIDs on trust (no auth required)
- ✅ **Real-time WebSocket connections** for instant event delivery
- ✅ **Broadcast or targeted messaging** (send to all users or specific user)
- ✅ **In-memory pub/sub** (simple and fast)
- ✅ **Multiple handshake types** (wave, high_five, fist_bump, etc.)
- ✅ **Connection management** with automatic cleanup
- ✅ **Active users tracking**

### API Endpoints

#### 1. WebSocket Connection
```
GET /handshake/ws?uid={user_id}
```
Establishes a WebSocket connection to receive real-time handshake events.

#### 2. Send Handshake
```
POST /handshake/send?uid={sender_uid}
Content-Type: application/json

{
  "type": "wave",           // required: handshake type
  "to_uid": "user123",      // optional: specific user (leave empty for broadcast)
  "message": "Hello!"       // optional: message
}
```

#### 3. Get Active Users
```
GET /handshake/active
```
Returns list of currently connected users.

### Event Format

When a handshake is sent, subscribers receive this JSON event:

```json
{
  "type": "wave",
  "from_uid": "sender123",
  "to_uid": "receiver456",    // optional
  "message": "Hello there!",  // optional
  "timestamp": "2025-09-07T05:49:32.123Z"
}
```

### Usage Examples

#### Connect via WebSocket (JavaScript)
```javascript
const socket = new WebSocket('ws://localhost:8080/handshake/ws?uid=myuser123');

socket.onmessage = function(event) {
    const handshake = JSON.parse(event.data);
    console.log(`${handshake.type} from ${handshake.from_uid}!`);
};
```

#### Send a Handshake (curl)
```bash
# Broadcast to all users
curl -X POST "http://localhost:8080/handshake/send?uid=myuser123" \
  -H "Content-Type: application/json" \
  -d '{"type":"wave","message":"Hello everyone!"}'

# Send to specific user
curl -X POST "http://localhost:8080/handshake/send?uid=myuser123" \
  -H "Content-Type: application/json" \
  -d '{"type":"high_five","to_uid":"friend456","message":"Great job!"}'
```

#### Get Active Users
```bash
curl -X GET "http://localhost:8080/handshake/active"
# Response: {"users":["user1","user2","user3"]}
```

### Available Handshake Types

- `wave` - 👋 Wave
- `high_five` - 🙏 High Five  
- `fist_bump` - 👊 Fist Bump
- `peace` - ✌️ Peace
- `thumbs_up` - 👍 Thumbs Up
- Or any custom string

### Demo

Open `handshake_demo.html` in your browser for a live interactive demo!

### Architecture

```
┌─────────────────┐    WebSocket    ┌─────────────────────┐
│   User Browser  │◄───────────────►│   PubSubService     │
└─────────────────┘                 │                     │
                                    │ ┌─────────────────┐ │
┌─────────────────┐    HTTP POST    │ │ In-Memory Store │ │
│   User App      │────────────────►│ │  uid -> [conn]  │ │
└─────────────────┘                 │ └─────────────────┘ │
                                    └─────────────────────┘
                                              │
                                              ▼
                                    ┌─────────────────────┐
                                    │ HandshakeService    │
                                    │ - SendHandshake()   │
                                    │ - GetActiveUsers()  │
                                    └─────────────────────┘
```

---

## Original Setup Instructions

## Setup

1. **Set up Supabase Database**
   - Create the users table with the following SQL:
   ```sql
   -- Add missing fields with length constraints
   ALTER TABLE public.users 
   ADD COLUMN username varchar(24) UNIQUE,
   ADD COLUMN name varchar(50);

   -- Create indexes for faster lookups
   CREATE INDEX idx_users_username ON public.users(username);
   CREATE INDEX idx_users_wallet_address ON public.users(wallet_address);
   ```

2. **Configure Environment**
   - Copy `.env.example` to `.env` and fill in your credentials:
   ```bash
   cp .env.example .env
   ```
   - Add your Stream Chat and Supabase credentials

3. **Install dependencies:**
   ```bash
   go mod download
   ```

4. **Run the server:**
   ```bash
   go run .
   ```

The server will start on port 8080 by default.

## API Documentation

Interactive API documentation is available via Swagger UI once the server is running:
- **Swagger UI**: http://localhost:8080/swagger/index.html
- **OpenAPI JSON**: http://localhost:8080/swagger/doc.json

### Regenerating Documentation
To regenerate the API documentation after making changes:
```bash
~/go/bin/swag init
```

## Chatbot Features

The API includes AI chatbot integration powered by OpenAI's ChatGPT:

### **Chatbot Endpoints:**
- `POST /chatbot/chat` - Chat with AI (specify model in request body)
- `GET /messages/channel/{channel_id}` - Get channel message history

### **Example Request:**
```json
{
  "channel_id": "general",
  "message": "Hello, what's the weather like?",
  "user_id": "user123",
  "model": "gpt-4"  // optional: "gpt-3.5-turbo" (default) or "gpt-4"
}
```

### **How It Works:**
1. **Context Loading**: The chatbot loads recent channel messages for context
2. **AI Processing**: Messages are sent to OpenAI with conversation history
3. **Database Storage**: Both user messages and AI responses are stored in Supabase
4. **Stream Integration**: Messages can be synced with Stream Chat channels

### **Message Types:**
- `user` - Messages from human users
- `assistant` - AI chatbot responses  
- `system` - System messages for context

## API Endpoints

### Health Check
- **GET** `/health` - Check if the server is running

### Authentication

#### Login
- **POST** `/auth/login`
- Body:
```json
{
  "username": "john_doe", // optional
  "wallet_address": "0x123..." // optional - provide either username or wallet_address
}
```
- Response:
```json
{
  "user": {
    "id": "uuid",
    "username": "john_doe",
    "name": "john_doe",
    "wallet_address": "0x123...",
    "profile_pic_url": "",
    "bio": "",
    "created_at": "2024-01-01T00:00:00Z"
  },
  "token": "jwt_token",
  "stream_token": "stream_chat_token"
}
```

#### Register
- **POST** `/auth/register`
- Body:
```json
{
  "username": "john_doe",
  "name": "John Doe",
  "wallet_address": "0x123...", // optional
  "profile_pic_url": "https://...", // optional
  "bio": "Hello world!" // optional
}
```
- Response: Same as login

### Stream Chat Integration

#### Generate Token
- **POST** `/stream/token`
- Body:
```json
{
  "user_id": "john_doe"
}
```
- Response:
```json
{
  "token": "stream_chat_token",
  "user_id": "john_doe"
}
```

#### Create/Update User
- **POST** `/stream/user`
- Body:
```json
{
  "id": "john_doe",
  "name": "John Doe",
  "email": "john@example.com",
  "image": "https://example.com/avatar.jpg"
}
```
- Response:
```json
{
  "message": "User created/updated successfully",
  "user_id": "john_doe"
}
```

## Frontend Integration

The frontend should:

1. Call `/auth/login` to authenticate the user
2. Use the returned `stream_token` to connect to Stream Chat
3. Use the JWT `token` for authenticated API calls

Example frontend connection:
```javascript
const response = await fetch('http://localhost:8080/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ username: 'john_doe' })
});

const { user, stream_token } = await response.json();

// Connect to Stream Chat
const chatClient = StreamChat.getInstance('your-api-key');
await chatClient.connectUser(user, stream_token);
```

## Environment Variables

- `STREAM_API_KEY` - Your Stream Chat API key
- `STREAM_SECRET` - Your Stream Chat secret  
- `JWT_SECRET` - Secret for signing JWT tokens
- `SUPABASE_URL` - Your Supabase project URL
- `SUPABASE_SERVICE_KEY` - Your Supabase service role key (full database access)
- `OPENAI_API_KEY` - Your OpenAI API key for ChatGPT integration
- `PORT` - Server port (default: 8080)

## Database Schema

Your Supabase database should have these tables:

**Users table:**
```sql
create table public.users (
  id uuid not null default gen_random_uuid (),
  created_at timestamp with time zone not null default now(),
  username varchar(24) unique,
  name varchar(50),
  wallet_address text null,
  profile_pic_url text null,
  bio text null,
  constraint users_pkey primary key (id)
);
```

**Messages table:**
```sql
create table public.messages (
  id uuid not null default gen_random_uuid (),
  created_at timestamp with time zone not null default now(),
  message_text text null,
  sender_id text null,
  channel_id text null,
  message_type text null,
  sender_username text null,
  type text null default 'text'::text,
  stream_message_id text null,
  reply_to_id uuid null,
  constraint messages_pkey primary key (id),
  constraint messages_reply_to_id_fkey foreign KEY (reply_to_id) references messages (id) on update CASCADE on delete CASCADE
);
```

## Note

This implementation uses Supabase for user storage and supports Web3-style authentication with wallet addresses. For production:
- Add rate limiting and security measures
- Implement proper error handling and logging
- Add user input validation
- Set up proper RLS policies in Supabase