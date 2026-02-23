# Geo-Location Based Server Routing

## Overview

Added geo-location routing functionality that allows directing clients to different servers based on their geographic location.

## Changes

### 1. Server Structure
Added a new `Region` field to the `Server` structure:

```go
type Server struct {
    Name     string `json:"name"`
    DNS      string `json:"dns"`
    Sessions int    `json:"sessions"`
    Enable   bool   `json:"enable"`
    Online   bool   `json:"online"`
    Region   string `json:"region"` // Region restriction, e.g., "RU" for Russia-only servers
}
```

### 2. Routing Logic (Universal)

The logic works automatically for **any country** without code changes:

#### Global Servers (`region: ""`):
- Available to **all clients** from any country
- Participate in the common pool for session monitoring and distribution
- Used as the main pool for countries without dedicated servers

#### Regional Servers (e.g., `region: "RU"`, `region: "CN"`, `region: "US"`):
- ⚠️ **ISOLATED** from other countries - not included in their server pool
- ⚠️ **DO NOT participate** in session monitoring for other countries
- ✅ Available **ONLY** to clients with matching `country_code` in POST request
- ✅ For their own country, they work as regular servers with session monitoring

#### Client Rules:
- Client with `country_code: "RU"` sees: **ONLY str9** (regional RU) - global servers excluded
- Client with `country_code: "IL"` sees: **ONLY global** servers - regional servers excluded
- Client without `country_code` sees: only global servers

**Key Logic:** If at least one regional server exists for a country, global servers are completely excluded for that country!

### 3. Server Filtering Algorithm

When receiving a POST request with `country_code`, the system applies two-stage filtering:

**Stage 1: Server Sorting**
```go
for each server in configuration:
    if server.Region == "":
        add server to globalServers
    else if server.Region == country_code:
        add server to regionalServers
    else:
        skip (server for different country)
```

**Stage 2: Pool Selection**
```go
if regionalServers exist:
    use ONLY regionalServers (global servers excluded)
else:
    use ONLY globalServers
```

**Key Point:** The presence of at least one regional server for a country **completely excludes** global servers from the pool for that country. This isolates regional traffic.

### 4. Functions

#### `getBestServerForCountry(countryCode string)`
New function that selects the optimal server based on the client's country code.

**Parameters:**
- `countryCode` - two-letter country code (ISO 3166-1 alpha-2), e.g., "RU", "US", "IL"

**Operation Logic:**
1. Filters servers by region based on country code
2. Selects server with minimum session count
3. For equal session counts, selects a random server

#### `getBestServer()`
Kept for backward compatibility. Calls `getBestServerForCountry("")`, meaning "no region restrictions".

### 5. POST /server
The `getServerByID` handler now automatically extracts the country code from the `Geo.CountryCode` field in the request body and uses it for server selection.

## Configuration

### Configuration Format
Example `conf.json` with multiple regions:

```json
{
  "str1": {
    "name": "str1",
    "dns": "str1.example.com",
    "sessions": 0,
    "enable": true,
    "online": true,
    "region": ""
  },
  "str2": {
    "name": "str2",
    "dns": "str2.example.com",
    "sessions": 0,
    "enable": true,
    "online": true,
    "region": ""
  },
  "str9": {
    "name": "str9",
    "dns": "str9.example.com",
    "sessions": 0,
    "enable": true,
    "online": true,
    "region": "RU"
  },
  "str10": {
    "name": "str10",
    "dns": "str10.example.com",
    "sessions": 0,
    "enable": true,
    "online": true,
    "region": "CN"
  },
  "str11": {
    "name": "str11",
    "dns": "str11.example.com",
    "sessions": 0,
    "enable": true,
    "online": true,
    "region": "US"
  }
}
```

### Region Parameters

- **`region: ""`** (empty string) - **global server**, available to all clients from any country
- **`region: "RU"`** - **regional server** for Russia, available **only** to clients with `country_code: "RU"`
- **`region: "CN"`** - **regional server** for China, available **only** to clients with `country_code: "CN"`
- **`region: "US"`** - **regional server** for USA, available **only** to clients with `country_code: "US"`

You can use any two-letter country codes (ISO 3166-1 alpha-2) to create regional pools. The logic applies automatically without code changes!

## Usage Examples

### Scenario 1: Client from Russia (RU)
```json
POST /server
{
  "geo": {
    "country_code": "RU",
    "city": "Moscow",
    "region": "Moscow"
  }
}
```
**Available servers in pool:** 
- ✅ `str9` (region: "RU") - **THE ONLY available**

**Unavailable servers (completely excluded):** 
- ❌ `str1`, `str2`, `str3`... (global) - **excluded because regional server exists for RU**
- ❌ `str10` (region: "CN")
- ❌ `str11` (region: "US")

**Result:** Returns **str9** (the only available regional server for RU)

### Scenario 2: Client from China (CN)
```json
POST /server
{
  "geo": {
    "country_code": "CN",
    "city": "Beijing",
    "region": "Beijing"
  }
}
```
**Available servers in pool:** 
- ✅ `str10` (region: "CN") - **THE ONLY available**

**Unavailable servers (completely excluded):** 
- ❌ `str1`, `str2`, `str3`... (global) - **excluded because regional server exists for CN**
- ❌ `str9` (region: "RU")
- ❌ `str11` (region: "US")

**Result:** Returns **str10** (the only regional server for CN)

### Scenario 3: Client from Israel (IL) - no dedicated regional servers
```json
POST /server
{
  "geo": {
    "country_code": "IL",
    "city": "Tel Aviv",
    "region": "Tel Aviv"
  }
}
```
**Available servers in pool:** 
- ✅ `str1` (region: "", global)
- ✅ `str2` (region: "", global)

**Unavailable servers (isolated):** 
- ❌ `str9` (region: "RU", **excluded** - not in pool for IL)
- ❌ `str10` (region: "CN", **excluded** - not in pool for IL)
- ❌ `str11` (region: "US", **excluded** - not in pool for IL)

**Important:** Client from Israel **does not participate** in monitoring and load balancing of regional servers. They simply don't exist in the system for them!

### Scenario 4: Client without country code or country not determined
```json
POST /server
{
  "geo": {
    "country_code": "",
    "city": "",
    "region": ""
  }
}
```
**Available servers in pool:** 
- ✅ `str1` (region: "", global)
- ✅ `str2` (region: "", global)

**Unavailable servers (isolated):** 
- ❌ `str9` (region: "RU")
- ❌ `str10` (region: "CN")
- ❌ `str11` (region: "US")

**Result:** All regional servers are **completely excluded** from the pool for clients without `country_code`

## Routing Table

For clarity, here's how servers are distributed among clients:

| Server | Region | 🇷🇺 RU | 🇨🇳 CN | 🇺🇸 US | 🇮🇱 IL | No country |
|--------|--------|--------|--------|--------|--------|------------|
| str1   | ""     | ❌     | ❌     | ❌     | ✅     | ✅         |
| str2   | ""     | ❌     | ❌     | ❌     | ✅     | ✅         |
| str9   | "RU"   | ✅     | ❌     | ❌     | ❌     | ❌         |
| str10  | "CN"   | ❌     | ✅     | ❌     | ❌     | ❌         |
| str11  | "US"   | ❌     | ❌     | ✅     | ❌     | ❌         |

**Conclusions:**
- Global servers (`region: ""`) are available **only** to countries without regional servers
- Regional servers are available **only** to their own clients
- **If a country has regional servers, global servers are excluded for it**
- Each country gets **EITHER** their regional servers **OR** global servers (not both!)

## Load Balancing with Multiple Regional Servers

If you have **multiple servers with the same region code**, they will automatically balance clients among themselves based on session count.

**Example configuration:**
```json
{
  "str9": {"region": "RU", "sessions": 10},
  "str9-backup": {"region": "RU", "sessions": 2},
  "str9-reserve": {"region": "RU", "sessions": 7}
}
```

**Request from Russian client:**
- All three servers (`str9`, `str9-backup`, `str9-reserve`) will be included in the pool
- **Global servers will be excluded** (because regional servers exist for RU)
- Server with minimum sessions will be selected: **`str9-backup`** (sessions: 2)

This allows you to scale regional servers horizontally without code changes while maintaining traffic isolation!

## Logging

The `getBestServerForCountry` function logs the following information:
- Selected server
- Server DNS
- Session count
- Server region
- Client country code

Example log:
```
DEBUG Selected server for request server=str9 dns=str9.example.com sessions=5 region=RU country_code=RU
```

## Backward Compatibility

All changes are backward compatible:
- `region` field is optional (defaults to empty string = global server)
- `getBestServer()` function continues to work without changes
- GET `/server` works without region filtering
- Old configurations without `region` field will continue to work
- **Universal logic** - no code changes required when adding new countries
