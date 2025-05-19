"""
Example Python function for the YouTube Serverless Platform
This function demonstrates a simple HTTP request handler
"""
import os
import json
import sys

def main():
    """
    Main function that processes input and returns a response
    """
    # Get input from environment variables
    name = os.environ.get('NAME', 'World')
    
    # Process the input
    message = f"Hello, {name}!"
    
    # Create a response object
    response = {
        "message": message,
        "timestamp": "2023-01-16T12:34:56Z",
        "environment": {
            "python_version": sys.version,
            "platform": sys.platform
        }
    }
    
    # Return the response as JSON
    print(json.dumps(response, indent=2))
    return response

if __name__ == "__main__":
    main()
