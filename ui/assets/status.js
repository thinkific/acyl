// url format: http://origin/some/path/status?id=990b38ff-17f1-4ea8-98a1-31bb2cad8b9b

/*
*  This page polls the status API endpoint (every pollingIntervalMilliseconds + jitter) and builds a table of
*  information and a tree visualization of the environment being built/upgraded.
*/

/* See status.html for API endpoint definitions */

const pollingIntervalMilliseconds = 750;

let done = false;
let failures = 0;
let updateInterval = setInterval(update, pollingIntervalMilliseconds);

// https://stackoverflow.com/questions/21294302/converting-milliseconds-to-minutes-and-seconds-with-javascript
function millisToMinutesAndSeconds(millis) {
    var minutes = Math.floor(millis / 60000);
    var seconds = ((millis % 60000) / 1000).toFixed(0);
    return (seconds == 60 ? (minutes+1) + ":00" : minutes + ":" + (seconds < 10 ? "0" : "") + seconds);
}

function updateConfig(cfg) {
    let sicon = document.getElementById("status-icon");
    let slinkbtnclass = "";
    switch (cfg.status) {
        case "pending":
            slinkbtnclass = "btn-outline-primary";
            sicon.style.color = "#0000ff";
            sicon.innerHTML = "\uf28b";
            break;
        case "done":
            slinkbtnclass = "btn-outline-success";
            sicon.style.color = "#00ff00";
            sicon.innerHTML = "\uf058";
            break;
        case "failed":
            slinkbtnclass = "btn-outline-danger";
            sicon.style.color = "#ff0000";
            sicon.innerHTML = "\uf071";
            break;
        default:
            slinkbtnclass = "btn-outline-warning";
            sicon.style.color = "#ffffff";
            sicon.innerHTML = "";
    }
    let slink = document.getElementById("rendered-status-link");
    slink.innerHTML = `<button class="btn ${slinkbtnclass}">${cfg.rendered_status.description}</button>`;
    slink.href = cfg.rendered_status.link_target_url;
    const repourl = `https://github.com/${cfg.triggering_repo}`;
    document.getElementById("trepo-link").text = repourl;
    document.getElementById("trepo-link").href = repourl;
    const prurl = `https://github.com/${cfg.triggering_repo}/pull/${cfg.pull_request}`;
    document.getElementById("trepo-pr-link").text = prurl;
    document.getElementById("trepo-pr-link").href = prurl;
    const userurl = `https://github.com/${cfg.github_user}`;
    document.getElementById("trepo-user-link").text = userurl;
    document.getElementById("trepo-user-link").href = userurl;

    document.getElementById("env-name").innerHTML = `<b>${cfg.env_name}</b>`;

    document.getElementById("trepo-branch").innerHTML = cfg.branch;
    document.getElementById("trepo-revision").innerHTML = cfg.revision;
    document.getElementById("event-type").innerHTML = cfg.type;
    document.getElementById("event-status").innerHTML = cfg.status;

    document.getElementById("config-processing-duration").innerHTML = cfg.processing_time;
    document.getElementById("event-started-time").innerHTML = cfg.started;

    let start = new Date(cfg.started).getTime();
    let end = new Date().getTime();

    if (cfg.completed !== null) {
        done = true;
        end = new Date(cfg.completed).getTime();
        document.getElementById("event-completed-time").innerHTML = cfg.completed;
    }

    document.getElementById("event-elapsed").innerHTML = millisToMinutesAndSeconds(end - start);
}

let tree, svg, diagonal = null;

// createTree creates the initial D3 tree w/o any node definitions after initial page load
function createTree() {

    let viewwidth = Math.max(document.documentElement.clientWidth, window.innerWidth || 0);

    let margin = {top: 50, right: 0, bottom: 0, left: 0},
        width = viewwidth - margin.right - margin.left,
        height = 400 - margin.top - margin.bottom;

    tree = d3.layout.tree()
        .nodeSize([4,4])
        .separation(function(a,b) { return a.parent == b.parent ? 25 : 20 });

    diagonal = d3.svg.diagonal()
        .projection(function (d) {
            return [d.x, d.y];
        });

    let h = height + margin.top + margin.bottom;
    let w = width + margin.right + margin.left;

    svg = d3.select("#envtree").append("svg")
        .attr("width", w)
        .attr("height", h)
        .append("g")
        .style("transform", "translate(50%, 12%)");
}

// stratify processes the flat object of nodes into a nested structure for D3
function stratify(treedata) {
    let out = [];
    const names = Object.keys(treedata);
    for (const name of names) {
        let obj = treedata[name];
        obj["name"] = name;
        out.push(obj);
    }
    return d3.stratify().id(function(d) { return d.name; }).parentId(function(d) { return d.parent; })(out);
}

// updateTree updates the existing D3 tree with the updated nodes
function updateTree(treedata) {

    let root = stratify(treedata);

    let nodes = tree.nodes(root).reverse(),
        links = tree.links(nodes);

    // Normalize for fixed-depth.
    nodes.forEach(function(d) {
        // Root node should be vertically offset to allow margin between the container and root node
        if (d.depth == 0) {
            d.y = 25;
        } else {
            // fixed depth of 90 pixels per tree level
            d.y = d.depth * 90;
        }
        let nttd = d3.select("body").append("div")
            .attr("class", "container")
            .append("div")
            .attr("id", `tooltip-${d.id}`)
            .attr("class", "tree-tooltip btn-group-vertical")
            .style("display", "none");

        nttd.append("h5")
            .attr("id", `tooltip-${d.id}-name`);

        nttd.append("button")
            .attr("type", "button")
            .attr("class", "tt-helm-btn btn btn-info btn-sm")
            .style("display", "none")
            .html(`<img src="https://helm.sh/img/helm.svg" height="16" width="16"> Chart Dependency`);

        nttd.append("button")
            .attr("type", "button")
            .style("display", "none")
            .attr("class", "tt-repo-btn btn btn-light btn-sm");

        nttd.append("button")
            .attr("type", "button")
            .style("display", "none")
            .attr("class", "tt-image-btn btn btn-primary btn-sm")
            .html("Image: Building");

        nttd.append("button")
            .attr("type", "button")
            .style("display", "none")
            .attr("class", "tt-chart-btn btn btn-secondary btn-sm")
            .html("Chart: Waiting");
    });

    function getSVGClass(d) {
        switch (d.data.chart.status) {
            case "installing":
            case "upgrading":
                return "spinning-svg";
        }
        return "";
    }

    function getCircleVisibility(d) {
        if (d.data.image !== null) {
            if (d.data.image.error) {
                return "hidden";
            }
        }
        switch (d.data.chart.status) {
            case "waiting":
            case "installing":
            case "upgrading":
                return "visible";
        }
        return "hidden";
    }

    function getCircleClass(d) {
        switch (d.data.chart.status) {
            case "installing":
            case "upgrading":
                return "spinning-circle";
        }
        return "";
    }

    function getCircleStroke(d) {
        switch (d.data.chart.status) {
            case "installing":
            case "upgrading":
                return "#2f3d4c";
        }
        return "#000000";
    }

    function getNodeIconVisibility(d) {
        if (d.data.image !== null) {
            if (d.data.image.error) {
                return "visible";
            }
        }
        switch (d.data.chart.status) {
            case "waiting":
            case "installing":
            case "upgrading":
                return "hidden";
        }
        return "visible";
    }

    function getNodeIconValue(d) {
        // https://fontawesome.com/cheatsheet/free/solid
        if (d.data.image !== null) {
            if (d.data.image.error) {
                return "\uf071";
            }
        }
        switch (d.data.chart.status) {
            case "done":
                return "\uf058";
            case "failed":
                return "\uf071";
        }
        return "\uf059";
    }

    function getNodeIconColor(d) {
        if (d.data.image !== null) {
            if (d.data.image.error) {
                return "#ff0000";
            }
        }
        switch (d.data.chart.status) {
            case "done":
                return "#00ff00";
            case "failed":
                return "#ff0000";
        }
        return "#000000";
    }

    // Declare the nodes
    let node = svg.selectAll("g.node")
        .data(nodes, function(d) { return d.id; });

    // Define the update selections

    node.select("circle")
        .attr("r", function(d) {
            return 10;
        })
        .attr("class", getCircleClass)
        .style("visibility", getCircleVisibility)
        .style("stroke", getCircleStroke)
        .style("fill", function(d) {
            switch (d.data.chart.status) {
                case "installing":
                case "upgrading":
                    return "#ffffff";
                case "waiting":
                case "done":
                    break;
                default:
                    console.log(`circle: got unknown chart status: ${d.data.chart.status}`);
            }
            return "gray";
        });

    node.select("g")
        .attr("class", getSVGClass);

    node.select("rect")
        .style("visibility", getNodeIconVisibility);

    node.select(".fas")
        .style("visibility", getNodeIconVisibility)
        .style("fill", getNodeIconColor)
        .text(getNodeIconValue);

    let tt = d3.selectAll(".tree-tooltip").data(nodes);

    tt.select("h5")
        .html(function(d) { return d.id; });

    tt.select(".tt-repo-btn")
        .style("display", function(d) {
            return (d.data.image === null) ? "none" : "block";
        })
        .html(function(d) {
            const txt = d.depth === 0 ? "Triggering Repo" : "Repo Dependency";
            return `<img src="https://github.com/favicon.ico" height="16" width="16"> ${txt}`;
        });

    tt.select(".tt-helm-btn")
        .style("display", function(d) {
            return (d.data.image === null) ? "block" : "none";
        });

    tt.select(".tt-image-btn")
        .style("display", function(d) {
            return (d.data.image === null) ? "none" : "block";
        })
        .attr("class", function(d) {
            if (d.data.image === null) {
                return "tt-image-btn btn btn-primary btn-sm";
            }
            if (d.data.image.completed === null) {
                return "tt-image-btn btn btn-primary btn-sm";
            }
            return (d.data.image.error) ? "tt-image-btn btn btn-danger btn-sm" : "tt-image-btn btn btn-success btn-sm";
        })
        .text(function(d) {
            if (d.data.image === null) {
                return "";
            }
            let start = new Date(d.data.image.started).getTime();
            let end = new Date().getTime();
            if (d.data.image.completed == null) {
                return `Image: Building... (${millisToMinutesAndSeconds(end - start)})`;
            }
            end = new Date(d.data.image.completed).getTime();
            const txt = (d.data.image.error) ? "Error" : "Done";
            return `Image: ${txt} (${millisToMinutesAndSeconds(end - start)})`;
        });

    tt.select(".tt-chart-btn")
        .style("display", function(d) {
            return (d.data.chart === null) ? "none" : "block";
        })
        .attr("class", function(d) {
            if (d.data.chart === null) {
                return "tt-chart-btn btn btn-secondary btn-sm";
            }
            switch (d.data.chart.status) {
            case "waiting":
                return "tt-chart-btn btn btn-secondary btn-sm";
            case "installing":
            case "upgrading":
                return "tt-chart-btn btn btn-warning btn-sm";
            case "done":
                return "tt-chart-btn btn btn-primary btn-sm";
            case "failed":
                return "tt-chart-btn btn btn-danger btn-sm";
            default:
                return "tt-chart-btn btn btn-secondary btn-sm";
            }
        })
        .text(function(d) {
            if (d.data.chart === null) {
                return "";
            }
            let start = d.data.chart.started !== null ? new Date(d.data.chart.started).getTime() : new Date().getTime();
            let end = new Date().getTime();
            switch (d.data.chart.status) {
                case "waiting":
                    if (d.data.image !== null && !d.data.image.error && d.data.image.completed === null) {
                        return "Chart: Waiting (image)";
                    } else {
                        if (d.children || d._children) {
                            return "Chart: Waiting (dependencies)";
                        }
                        return "Chart: Waiting";
                    }
                case "installing":
                    return `Chart: Installing (${millisToMinutesAndSeconds(end - start)})`;
                case "upgrading":
                    return `Chart: Upgrading (${millisToMinutesAndSeconds(end - start)})`;
                case "done":
                    end = new Date(d.data.chart.completed).getTime();
                    return `Chart: Done (${millisToMinutesAndSeconds(end - start)})`;
                case "failed":
                    end = new Date(d.data.chart.completed).getTime();
                    return `Chart: Failed (${millisToMinutesAndSeconds(end - start)})`;
                default:
                    return `Chart: Unknown (${d.data.chart.status})`;
            }
        });

    // Nodes should never be removed, but if they go missing from the model remove them
    node.exit().remove();

    // Define node entry
    let nodeEnter = node.enter().append("g")
        .on("mouseover", function(d) {
            d3.select(`#tooltip-${d.id}`)
                .style("display", "block")
                .transition()
                .duration(300)
                .style("opacity", 1);
        })
        .on("mousemove", function(d){
            d3.select(`#tooltip-${d.id}`)
                .style("left", (d3.event.pageX ) + "px")
                .style("top", (d3.event.pageY) + "px");
        })
        .on("mouseout", function(d) {
            d3.select(`#tooltip-${d.id}`)
                .transition()
                .duration(300)
                .style("opacity", 1e-6)
                .style("display", "none");
        })
        .attr("class", "node")
        .attr("id", function(d) { return d.id; })
        .attr("transform", function(d) {
            return "translate(" + d.x + "," + d.y + ")"; });

    nodeEnter.append("g")
        .attr("class", getSVGClass)
        .append("circle")
        .attr("r", function(d) {
            return 10;
        })
        .attr("class", getCircleClass)
        .style("visibility", getCircleVisibility)
        .style("stroke", getCircleStroke);

    // this is an opaque box behind the node icon to prevent the path from being visible in the transparent parts of the icon
    nodeEnter.append("rect")
        .attr("x", function(d) { return this.parentNode.getBBox().x;})
        .attr("y", function(d, i) { return  this.parentNode.getBBox().y })
        .attr("width", function(d) { return this.parentNode.getBBox().width;})
        .attr("height", function(d) {return 20;})
        .style("fill", "#ffffff")
        .style("visibility", getNodeIconVisibility);

    nodeEnter.append("text")
        .attr("class", "fas")
        .attr("x", 0)
        .attr("y", 10)
        .attr("text-anchor", "middle")
        .attr("font-family", "Font Awesome 5 Free")
        .attr("font-weight", 900)
        .attr("font-size", 24)
        .style("opacity", 1)
        .style("visibility", getNodeIconVisibility)
        .style("fill", getNodeIconColor)
        .text(getNodeIconValue);

    nodeEnter.append("text")
        .attr("class", "node-label")
        .attr("y", function(d) {
            return d.children || d._children ? -26 : 20; })
        .attr("dy", ".35em")
        .attr("text-anchor", "middle")
        .attr("font-weight", 700)
        .text(function(d) {
            // root node label should not be elided
            if (d.depth === 0) {
                return d.id;
            }
            let name = d.id;
            if (name.length <= 10) {
                return name;
            }
            return name.substring(0, 9) + "...";
        })
        .style("fill-opacity", 1);

    // Declare the links
    let link = svg.selectAll("path.link")
        .data(links, function(d) { return d.target.id; });

    // Enter the links.
    link.enter().insert("path", "g")
        .attr("class", "link")
        .attr("d", diagonal);
}

function updateLogs(logs) {
    document.getElementById("logsContainer").innerHTML = logs.join("<br \>\n");
    let objDiv = document.getElementById("logsContainer");
    objDiv.scrollTop = objDiv.scrollHeight;
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// https://stackoverflow.com/questions/4959975/generate-random-number-between-two-numbers-in-javascript
function randomIntFromInterval(min, max) { // min and max included
    return Math.floor(Math.random() * (max - min + 1) + min);
}

async function update() {
    let req = new XMLHttpRequest(), req2 = new XMLHttpRequest();

    req.open('GET', statusEndpoint, true);
    req.onload = function(e) {
        if (req.status !== 200) {
            failures++;
            console.log(`event status request failed (${failures}): ${req.status}: ${req.responseText}`);
            if (failures >= 10) {
                console.log(`API failures exceed limit: ${failures}: aborting update`);
                clearInterval(updateInterval);
            }
            return;
        }

        const data = JSON.parse(req.response);

        if (data.hasOwnProperty('config')) {
            updateConfig(data.config);
        } else {
            console.log("event status missing config element");
            return
        }

        if (data.hasOwnProperty('tree')) {
            if (Object.keys(data.tree).length > 0) {
                updateTree(data.tree);
            }
        } else {
            console.log("event status missing tree element");
        }

        if (done) {
            clearInterval(updateInterval);
        }
    };
    req.onerror = function(e) {
        console.error(`error getting status endpoint: ${req.statusText}`);
    };

    req2.open('GET', logsEndpoint, true);
    req2.setRequestHeader("Acyl-Log-Key", logKey);
    req2.onload = function(e) {
        if (req2.status !== 200) {
            failures++;
            console.log(`event logs request failed (${failures}): ${req2.status}: ${req2.responseText}`);
            if (failures >= 10) {
                console.log(`API failures exceed limit: ${failures}: aborting update`);
                clearInterval(updateInterval);
            }
            return;
        }

        const data2 = JSON.parse(req2.response);

        if (data2 !== null) {
            updateLogs(data2);
        }
    };
    req2.onerror = function(e) {
        console.error(`error getting event log endpoint: ${req2.statusText}`);
    };

    // add some jitter to make the display seem smoother
    await sleep(randomIntFromInterval(1, 500));

    req.send(null);
    req2.send(null);
}

document.addEventListener("DOMContentLoaded", function(){
    createTree();
    update();
});
