import os
import json
import base64
import requests
from io import BytesIO
from PIL import Image, ImageFilter, ImageOps

def main():
    # Get input from environment variables
    image_url = os.environ.get('IMAGE_URL', 'https://plus.unsplash.com/premium_photo-1747135794086-280753a24793?q=80&w=1935&auto=format&fit=crop&ixlib=rb-4.1.0&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D')
    width = int(os.environ.get('WIDTH', '300'))
    height = int(os.environ.get('HEIGHT', '300'))

    if not image_url:
        print(json.dumps({
            "error": "No image URL provided",
            "status": "error"
        }))
        return

    try:
        # Download the image
        response = requests.get(image_url)
        response.raise_for_status()

        # Open the image
        image = Image.open(BytesIO(response.content))

        # Process the image
        # 1. Resize
        image = image.resize((width, height))

        # 2. Convert to grayscale
        image = ImageOps.grayscale(image)

        # 3. Apply a blur filter
        image = image.filter(ImageFilter.GaussianBlur(radius=2))

        # Convert the processed image to base64
        buffered = BytesIO()
        image.save(buffered, format="JPEG")
        img_str = base64.b64encode(buffered.getvalue()).decode('utf-8')

        # Create response
        response = {
            "original_url": image_url,
            "processed_image": img_str,
            "transformations": ["resize", "grayscale", "blur"],
            "dimensions": {
                "width": width,
                "height": height
            },
            "status": "success"
        }

        # Return the response
        print(json.dumps(response))

    except Exception as e:
        print(json.dumps({
            "error": str(e),
            "status": "error"
        }))

if __name__ == "__main__":
    main()
