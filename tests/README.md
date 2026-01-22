# MOHAA Stats API Tests

This directory contains tests for the MOHAA Stats API login flow.

## Test Files

| File | Description |
|------|-------------|
| `login_test.go` | Go integration tests using the standard `testing` package |
| `integration_login.sh` | Shell script for quick integration testing |

## Running Tests

### Go Tests

```bash
# Run all tests
cd /home/elgan/dev/opm-stats-system
go test -v ./tests/...

# Run specific test
go test -v ./tests/... -run TestLoginFlow

# With custom API URL
API_URL=http://api:8080 go test -v ./tests/...
```

### Shell Integration Tests

```bash
# Run integration tests
./tests/integration_login.sh

# With custom settings
API_URL=http://localhost:8080 FORUM_USER_ID=5 ./tests/integration_login.sh
```

## Test Coverage

### Login Flow Tests (`TestLoginFlow`)

1. **APIHealth** - Verifies API is responsive
2. **GenerateToken** - Tests token generation for SMF user
3. **VerifyToken** - Simulates `tracker.scr` `/login` command
4. **TokenReuseBlocked** - Ensures tokens can only be used once
5. **InvalidTokenRejected** - Tests rejection of invalid tokens

### Token Generation Tests (`TestTokenGeneration`)

1. **RequiresForumUserID** - Validates forum_user_id is required
2. **RegenerateRevokesOldToken** - Tests that regenerating revokes old tokens

### Validation Tests (`TestVerifyTokenValidation`)

1. **RequiresToken** - Validates token field is required
2. **InvalidJSON** - Tests malformed JSON handling

## Login Flow

The complete login flow being tested:

```
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│   SMF Forum     │       │    Go API       │       │   Game Server   │
│  (Profile Page) │       │   (:8080)       │       │  (tracker.scr)  │
└────────┬────────┘       └────────┬────────┘       └────────┬────────┘
         │                         │                         │
         │ POST /auth/device       │                         │
         │ {forum_user_id: 1}      │                         │
         │────────────────────────>│                         │
         │                         │                         │
         │ {user_code: "MOH-12345"}│                         │
         │<────────────────────────│                         │
         │                         │                         │
         │       User types: /login MOH-12345                │
         │                         │                         │
         │                         │ POST /auth/verify       │
         │                         │ {token, player_guid...} │
         │                         │<────────────────────────│
         │                         │                         │
         │                         │ {valid: true,           │
         │                         │  forum_user_id: 1}      │
         │                         │────────────────────────>│
         │                         │                         │
         │                   Identity Linked!                │
         └─────────────────────────┴─────────────────────────┘
```

## Database Tables Used

- `login_tokens` - Stores generated tokens
- `login_token_history` - Records all login attempts
- `smf_user_mappings` - Links forum users to game GUIDs
