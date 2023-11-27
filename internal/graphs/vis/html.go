package vis

// TODO: Tidy, add options to JS?

var html = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <style>
        * {
            margin: 0;
        }
        #mynetwork {
            width: 100vw;
            height: 100vh;
        }
    </style>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <script type="text/javascript"
      src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
  </head>
  <body>
    <div id="mynetwork"></div>
    <script type="text/javascript">
let nodesAndEdges = [%s
];

var container = document.getElementById("mynetwork");

var data = {
  nodes: [],
  edges: [],
};

var options = {
  physics: {
    enabled: true,
    solver: 'barnesHut',
    repulsion: {
      nodeDistance: 200,
    },
    barnesHut: {
      gravitationalConstant: -10_000,
    }
  }
};
var network = new vis.Network(container, data, options);

let index = 0;

function addItem() {
    if (index < nodesAndEdges.length) {
        const item = nodesAndEdges[index];
        const dataType = item.type;

        if (dataType === "node") {
            network.body.data.nodes.add(item.data);
        } else if (dataType === "edge") {
            network.body.data.edges.add(item.data);
        }

        index++;
        setTimeout(addItem, 50); // milliseconds
    }
}

// Call the function to start the iteration
addItem();
        </script>
  </body>
</html>`
