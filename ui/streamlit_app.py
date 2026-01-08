import os
import requests
import streamlit as st

st.set_page_config(page_title="DWOP", layout="centered")

st.title("DWOP Workflow")

default_base = os.getenv("DWOP_API_BASE", "http://localhost:8080")
api_base = st.text_input("API Base URL", value=default_base)

st.caption("Endpoints used: POST /upload, POST /update, POST /cancel")


def _post(url: str, *, data=None, files=None, timeout=120):
    try:
        r = requests.post(url, data=data, files=files, timeout=timeout)
        return r
    except requests.RequestException as e:
        st.error(str(e))
        return None


def _show_response(r: requests.Response | None):
    if r is None:
        return
    st.write("Status:", r.status_code)
    content_type = r.headers.get("content-type", "")
    if "application/json" in content_type:
        try:
            st.json(r.json())
            return
        except Exception:
            pass
    st.text(r.text)


tab_upload, tab_update, tab_cancel = st.tabs(["Upload", "Update", "Cancel"])

with tab_upload:
    st.subheader("Upload workflow")
    wf = st.file_uploader("Workflow file", key="upload_wf")
    req = st.file_uploader("requirements file", key="upload_req")

    if st.button("Upload", type="primary", disabled=not (wf and req)):
        files = {
            "file": (wf.name, wf.getvalue(), "application/octet-stream"),
            "requirements": (req.name, req.getvalue(), "text/plain"),
        }
        r = _post(f"{api_base}/upload", files=files)
        _show_response(r)

with tab_update:
    st.subheader("Update workflow")
    workflow_id = st.text_input("workflowId", key="update_workflow_id")
    wf = st.file_uploader("New workflow file", key="update_wf")
    req = st.file_uploader("New requirements file", key="update_req")

    if st.button("Update", type="primary", disabled=not (workflow_id and wf and req)):
        files = {
            "file": (wf.name, wf.getvalue(), "application/octet-stream"),
            "requirements": (req.name, req.getvalue(), "text/plain"),
        }
        data = {"workflowId": workflow_id}
        r = _post(f"{api_base}/update", data=data, files=files)
        _show_response(r)

with tab_cancel:
    st.subheader("Cancel workflow")
    workflow_id = st.text_input("workflowId", key="cancel_workflow_id")

    if st.button("Cancel", type="primary", disabled=not workflow_id):
        data = {"workflowId": workflow_id}
        r = _post(f"{api_base}/cancel", data=data, timeout=30)
        _show_response(r)
