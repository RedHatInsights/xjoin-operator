FROM python:3.10.2

COPY . .

RUN pip install --upgrade pip && \
    pip install pipenv && \
    pipenv install --system --deploy --ignore-pipfile

CMD bash -c 'alembic upgrade head'
