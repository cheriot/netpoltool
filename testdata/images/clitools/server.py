from flask import Flask
import json
import os

app = Flask(__name__)


@app.route('/')
def pod_info():
    prefix = 'PUBLIC_'
    public_vars = dict(filter(lambda elem: elem[0].startswith(prefix), os.environ.items()))
    output_vars = { k.replace(prefix, '').lower(): v for k, v in public_vars.items() }
    return json.dumps(output_vars)