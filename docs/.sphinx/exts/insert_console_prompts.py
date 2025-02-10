import re
from docutils.nodes import Node
from pathlib import Path
from sphinx.application import Sphinx
from sphinx.directives.code import CodeBlock
from sphinx.util.typing import ExtensionMetadata


DOCS = (Path.cwd() / __file__).parents[2]

PROMPT = re.compile(r"\S*[#$](\s.*)?")
OUTPUT = re.compile(r"\s.*|")


class InsertPromptsCodeBlock(CodeBlock):
    def run(self) -> list[Node]:
        if self.arguments == ["console"]:
            # Only change files typically viewed on GitHub (docs/*.rst).
            source, _ = self.get_source_info()
            docs = (Path.cwd() / source).parent
            if docs.samefile(DOCS):
                insert_prompts(self.content)

        return super().run()


def insert_prompts(content: list[str]) -> None:
    """Prepend a shell prompt to lines which appear to be commands."""
    continued = False
    for i, line in enumerate(content):
        # Check for \ at end of previous line.
        skip = continued
        continued = line.endswith("\\")
        if skip:
            continue

        # Avoid replacing existing prompts or changing command output.
        if PROMPT.fullmatch(line) or OUTPUT.fullmatch(line):
            continue

        content[i] = f"$ {line}"


def setup(app: Sphinx) -> ExtensionMetadata:
    app.add_directive("code-block", InsertPromptsCodeBlock, override=True)

    return {
        "version": "0.1",
        "parallel_read_safe": True,
        "parallel_write_safe": True,
    }
