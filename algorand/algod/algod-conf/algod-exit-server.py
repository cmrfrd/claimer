import os
import subprocess
from bottle import route, run, request, abort

@route('/', method=["POST"])
def algod_exit():
    token = request.get_header("X-Algo-API-Token")
    if token == os.environ["ALGORAND_CLAIMER_ALGOD_TOKEN"]:
        cp = subprocess.run(["goal", "node", "stop"], universal_newlines=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        print(cp.stdout)
    else:
        abort(401, "ERROR: Invalid token")

    return "Success!"

run(host='0.0.0.0', port=os.environ["SHUTDOWN_PORT"])
