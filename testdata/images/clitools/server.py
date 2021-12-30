from flask import Flask
import json
import os

app = Flask(__name__)


@app.route('/')
def hello():
    env_vars = dict(filter(lambda elem: elem[0].startswith('POD_'), os.environ.items()))
    return json.dumps(env_vars)
