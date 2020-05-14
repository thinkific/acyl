// setenvlist replaces the table body with tbody
function setenvlist(tbody) {
    let oldtbody = document.getElementById("envlist-tbody");
    if (tbody === null) {
        return;
    }
    oldtbody.parentNode.replaceChild(tbody, oldtbody);
    tbody.id = "envlist-tbody";
}

// envrow creates a table row from an environment object
function envrow(env) {
    let tr = document.createElement("tr");
    tr.id = `envlist-${env.env_name}`;
    let tdrepo = document.createElement("td");
    tdrepo.className = "text-left";
    tdrepo.innerHTML = env.repo;
    tr.appendChild(tdrepo);
    let tdpr = document.createElement("td");
    tdpr.className = "text-left";
    tdpr.innerHTML = env.pull_request;
    tr.appendChild(tdpr)
    let tdname = document.createElement("td");
    tdname.className = "text-left";
    tdname.innerHTML = `<a href="${apiBaseURL}/ui/env/${env.env_name}">${env.env_name}</a>`;
    tr.appendChild(tdname);
    let tdlastevent = document.createElement("td");
    tdlastevent.className = "text-left";
    tdlastevent.innerHTML = env.last_event;
    tr.appendChild(tdlastevent);
    let tdstatus = document.createElement("td");
    tdstatus.className = "text-left";
    tr.appendChild(tdstatus);
    // color whole rows on non-success envs
    switch (env.status) {
        case "success":
            tdstatus.innerHTML = `<span class="badge badge-success">Success</span>`;
            break;
        case "failed":
            tr.className = "table-danger";
            tdstatus.innerHTML = `<span class="badge badge-danger">Failed</span>`;
            break;
        case "pending":
            tr.className = "table-warning";
            tdstatus.innerHTML = `<span class="badge badge-warning">Pending</span>`;
            break;
        case "destroyed":
            tr.className = "table-active"; // "active" colors the row gray
            tdstatus.innerHTML = `<span class="badge badge-secondary">Destroyed</span>`;
            break;
        default:
            tr.className = "table-active";
            tdstatus.innerHTML = `<span class="badge badge-secondary">Unknown</span>`;
            break;
    }
    return tr;
}

function renderenvlist(envs) {
    envs.sort(function(x, y) {
        const date1 = new Date(x.last_event);
        const date2 = new Date(y.last_event);
        return date2 - date1;
    });
    let tbody = document.createElement("tbody");
    for (let i = 0; i < envs.length; i++) {
        let row = envrow(envs[i]);
        tbody.appendChild(row);
    }
    setenvlist(tbody);
}

function setFormValues() {
    localStorage.setItem("filterIndex", document.getElementById("envgroup").selectedIndex);
    localStorage.setItem("historyIndex", document.getElementById("history").selectedIndex);
    localStorage.setItem("incDestroyedBool", document.getElementById("destroyed").checked);
}

function getFormValues() {
    let fidx = localStorage.getItem("filterIndex");
    if (fidx === null) {
        fidx = "0";
    }
    let tidx = localStorage.getItem("historyIndex");
    if (tidx === null) {
        tidx = "0";
    }
    let dbool = localStorage.getItem("incDestroyedBool");
    if (dbool === null) {
        dbool = "false";
    }
    document.getElementById("envgroup").selectedIndex = fidx;
    document.getElementById("history").selectedIndex = tidx;
    document.getElementById("destroyed").checked = (dbool === "true");
}

function update(allenvs, hist, destroyed) {
    let req = new XMLHttpRequest();

    req.open('GET', `${apiBaseURL}/v2/userenvs?history=${hist}h&include_destroyed=${destroyed}&allenvs=${allenvs}`, true);
    req.onload = function (e) {
        if (req.status !== 200) {
            console.log(`userenvs request failed: ${req.status}: ${req.responseText}`);
            return;
        }

        const data = JSON.parse(req.response);
        if (!Array.isArray(data)) {
            console.log(`userenvs received unexpected data (wanted array): ${data}`);
            return;
        }
        setFormValues();
        renderenvlist(data);
    };
    req.onerror = function (e) {
        console.error(`error getting userenvs endpoint: ${req.statusText}`);
    };
    req.send(null);
}

// gethistory returns the selected environment history length in hours
function gethistory() {
    let hist = document.getElementById("history");
    switch (hist.selectedIndex) {
        case 0:
            return 7 * 24;
        case 1:
            return 14 * 24;
        case 2:
            return 30 * 24;
        case 3:
            return 90 * 24;
        default:
            return 0;
    }
}

function getdestroyed() {
    return document.getElementById("destroyed").checked;
}

function getallenvs() {
    return document.getElementById("envgroup").selectedIndex === 1;
}

document.addEventListener("DOMContentLoaded", function(){
    getFormValues();
    document.getElementById("refreshbtn").addEventListener('click', function(e){
        e.preventDefault();
        update(getallenvs(), gethistory(), getdestroyed());
    });
    update(getallenvs(), gethistory(), getdestroyed());
});