"""
Thanks ChatGPT!

Usage:
Place out.json in this directory
Run `python json2doy.py`
Open out.dot, copy the text, paste into your favourite graphviz viewer
I like https://dreampuf.github.io/GraphvizOnline/
"""

import json


def json_to_dot(json_data):
    dot_str = "digraph WebsiteConnections {\n"

    for website, connections in json_data.items():
        # GPT struggled here, had to manually intervene and create connections_string
        connections_string = ", ".join(f'"{item}"' for item in connections)
        dot_str += f'  "{website}" -> {{ {connections_string} }};\n'

    dot_str += "}"
    return dot_str


def main():
    with open("out.json", "r") as json_file:
        data = json.load(json_file)

    dot_data = json_to_dot(data)

    with open("out.dot", "w") as dot_file:
        dot_file.write(dot_data)


if __name__ == "__main__":
    main()
