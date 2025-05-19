FROM python:3.9-slim
WORKDIR /app
COPY . .

# Test only dependency installation first
RUN if [ -f requirements.txt ]; then pip install -r requirements.txt; fi

# Test the wrapper script generation
RUN echo '#!/bin/sh\npython test.py "$@"' > /app/wrapper.sh && \
    chmod +x /app/wrapper.sh

CMD ["/app/wrapper.sh"]