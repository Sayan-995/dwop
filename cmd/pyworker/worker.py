import subprocess
import sys
import os
import urllib.request
import json
import traceback

TERMINATION_LOG = "/dev/termination-log"

REQUIRED_ENVS = (
    "CODE_URL",
    "REQ_URL",
    "PRED_URLS_JSON",
    "FUNC_ARG_MAP_JSON",
    "OUTPUT_SIGNED_URL",
)

def write_termination_log(msg:str):
    with open(TERMINATION_LOG,"w") as f:
        f.write(msg)


def read_termination_log() -> str:
    try:
        with open(TERMINATION_LOG, "r") as f:
            return f.read()
    except Exception:
        return ""


def read_tail(path: str, limit: int) -> str:
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            s = f.read()
            return s[-limit:]
    except Exception:
        return ""


def install_dependencies():
    subprocess.check_call(
        [sys.executable, "-m", "pip", "install", "-r", "requirements.txt"]
    )    
    
def run_task():
    result = subprocess.run(
        [sys.executable, "/app/task.py"],
        cwd="/app",
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    with open("output.txt", "w") as f:
        f.write(result.stdout)
    with open("error.txt","w")as f:
        f.write(result.stderr)

    if result.stdout:
        sys.stdout.write(result.stdout)
    if result.stderr:
        sys.stderr.write(result.stderr)

    if result.returncode != 0:
        write_termination_log(
            json.dumps(
                {
                    "stage": "run_task",
                    "exit_code": result.returncode,
                    "stdout_tail": result.stdout[-2000:],
                    "stderr_tail": result.stderr[-2000:],
                },
                ensure_ascii=False,
            )
        )
        raise RuntimeError(f"task.py failed with exit code {result.returncode}")

def get_content(url: str) -> bytes:
    if not url:
        raise ValueError("missing url")
    req = urllib.request.Request(url, method="GET")
    with urllib.request.urlopen(req, timeout=30) as r:
        return r.read()

def put_content(url: str, data: bytes):
    if not url:
        raise ValueError("missing output upload url")
    req = urllib.request.Request(
        url,
        data=data,
        method="PUT",
        headers={
            "Content-Type": "application/octet-stream",
            "Content-Length": str(len(data)),
        },
    )
    with urllib.request.urlopen(req, timeout=30) as r:
        _ = r.read()

def main():
    try:
        missing = [k for k in REQUIRED_ENVS if not os.getenv(k)]
        if missing:
            raise ValueError(f"missing required envs: {','.join(missing)}")

        code_url = os.getenv("CODE_URL")
        req_url = os.getenv("REQ_URL")
        pred_urls_json = os.getenv("PRED_URLS_JSON")
        func_arg_map_json = os.getenv("FUNC_ARG_MAP_JSON")
        output_url = os.getenv("OUTPUT_SIGNED_URL")

        user_code = get_content(code_url)
            
        with open("requirements.txt","wb")as f:
            f.write(get_content(req_url))
            
        pred_urls = json.loads(pred_urls_json) if pred_urls_json else {}
        func_arg_map = json.loads(func_arg_map_json) if func_arg_map_json else {}

        for pred_task, signed_url in pred_urls.items():
            arg_name = func_arg_map.get(pred_task)
            if not arg_name:
                raise ValueError(f"missing arg mapping for predecessor: {pred_task}")
            result = get_content(signed_url)
            with open(arg_name, "wb") as f:
                f.write(result)

        with open("task.py","wb")as f:
            f.write(user_code)
        
        install_dependencies()
        run_task()
        with open("output.txt","rb") as f:
            output=f.read()
        put_content(output_url,output)
    except Exception as e:
        tb = traceback.format_exc()
        existing = read_termination_log()
        if '"stage": "run_task"' not in existing:
            payload = {
                "stage": "worker",
                "error": str(e),
                "traceback": tb[-3000:],
                "task_stdout_tail": read_tail("output.txt", 2000),
                "task_stderr_tail": read_tail("error.txt", 2000),
            }
            write_termination_log(json.dumps(payload, ensure_ascii=False))
        sys.stderr.write(tb)
        sys.exit(1)
        
if __name__=="__main__":
    main()