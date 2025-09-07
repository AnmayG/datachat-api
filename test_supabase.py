#!/usr/bin/env python3

import os
import json
import requests
from datetime import datetime
import uuid

def load_env():
    """Load environment variables from .env file"""
    env_vars = {}
    try:
        with open('.env', 'r') as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith('#'):
                    key, value = line.split('=', 1)
                    env_vars[key] = value
    except FileNotFoundError:
        print("No .env file found")
    return env_vars

def test_supabase():
    # Load environment variables
    env_vars = load_env()
    
    url = env_vars.get('SUPABASE_URL') or os.getenv('SUPABASE_URL')
    key = env_vars.get('SUPABASE_SERVICE_KEY') or os.getenv('SUPABASE_SERVICE_KEY')
    
    if not url or not key:
        print("ERROR: Missing SUPABASE_URL or SUPABASE_SERVICE_KEY")
        return
    
    print(f"Using URL: {url}")
    print(f"Service key length: {len(key)}")
    print(f"Key prefix: {key[:20]}...")
    
    headers = {
        'apikey': key,
        'Authorization': f'Bearer {key}',
        'Content-Type': 'application/json',
        'Prefer': 'return=representation'
    }
    
    # Test 1: Simple select
    print("\n=== TEST 1: SELECT FROM USERS ===")
    response = requests.get(f"{url}/rest/v1/users", headers=headers)
    print(f"Status: {response.status_code}")
    print(f"Response: {response.text}")
    
    # Test 2: Insert test data
    print("\n=== TEST 2: INSERT TEST USER ===")
    test_user = {
        "id": str(uuid.uuid4()),
        "username": f"testuser_{int(datetime.now().timestamp())}",
        "name": "Test User Python"
    }
    
    print(f"Inserting: {json.dumps(test_user)}")
    response = requests.post(f"{url}/rest/v1/users", 
                           headers=headers, 
                           json=test_user)
    print(f"Status: {response.status_code}")
    print(f"Response: {response.text}")
    print(f"Headers: {dict(response.headers)}")
    
    # Test 3: Try with different prefer header
    print("\n=== TEST 3: INSERT WITH MINIMAL PREFER ===")
    headers['Prefer'] = 'return=minimal'
    test_user2 = {
        "id": str(uuid.uuid4()),
        "username": f"testuser2_{int(datetime.now().timestamp())}",
        "name": "Test User Python 2"
    }
    
    response = requests.post(f"{url}/rest/v1/users", 
                           headers=headers, 
                           json=test_user2)
    print(f"Status: {response.status_code}")
    print(f"Response: {response.text}")
    
    # Test 4: Check if users were created
    print("\n=== TEST 4: VERIFY INSERTS ===")
    headers['Prefer'] = 'return=representation'
    response = requests.get(f"{url}/rest/v1/users?id=eq.{test_user['id']}", headers=headers)
    print(f"First user lookup - Status: {response.status_code}")
    print(f"Response: {response.text}")
    
    response = requests.get(f"{url}/rest/v1/users?id=eq.{test_user2['id']}", headers=headers)
    print(f"Second user lookup - Status: {response.status_code}")
    print(f"Response: {response.text}")

if __name__ == "__main__":
    test_supabase()