#!/usr/bin/env python3
import requests

def get_current_ip():
    try:
        response = requests.get('https://api.ipify.org?format=json')
        return response.json()['ip']
    except Exception as e:
        print(f"Error: {e}")
        return None

if __name__ == "__main__":
    print(get_current_ip()) 