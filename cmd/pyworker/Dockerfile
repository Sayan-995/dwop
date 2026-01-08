FROM python:3.11
WORKDIR /app
COPY cmd/pyworker/worker.py /app/worker.py
ENTRYPOINT [ "python","/app/worker.py" ]