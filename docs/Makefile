# Minimal makefile for Sphinx documentation
#

# You can set these variables from the command line, and also
# from the environment for the first two.
SPHINXDIR     = .sphinx
SPHINXOPTS    ?= -c . -d $(SPHINXDIR)/.doctrees
SPHINXBUILD   ?= sphinx-build
SOURCEDIR     = .
BUILDDIR      = _build
VENVDIR       = $(SPHINXDIR)/venv
PA11Y         = $(SPHINXDIR)/node_modules/pa11y/bin/pa11y.js
VENV          = $(VENVDIR)/bin/activate


# Put it first so that "make" without argument is like "make help".
.PHONY: help
help:
	@echo "\n" \
        "--------------------------------------------------------------- \n" \
        "* watch, build and serve the documentation: make run \n" \
        "* only build: make html \n" \
        "* only serve: make serve \n" \
        "* clean built doc files: make clean-doc \n" \
        "* clean full environment: make clean \n" \
        "* check links: make linkcheck \n" \
        "* check spelling: make spelling \n" \
        "* check inclusive language: make woke \n" \
        "* check accessibility: make pa11y \n" \
        "* other possible targets: make <press TAB twice> \n" \
        "--------------------------------------------------------------- \n"

.PHONY: full-help
full-help: $(VENVDIR)
	@. $(VENV); $(SPHINXBUILD) -M help "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)
	@echo "\n\033[1;31mNOTE: This help texts shows unsupported targets!\033[0m"
	@echo "Run 'make help' to see supported targets."


# Must not assume the venv module is available because Reasons (capital R):
# https://bugs.launchpad.net/ubuntu/+source/python3.4/+bug/1290847
$(SPHINXDIR)/requirements.txt:
	python3 build_requirements.py
	python3 -c "import venv" || sudo apt install python3-venv


# If requirements are updated, venv should be rebuilt and timestamped.
$(VENVDIR): $(SPHINXDIR)/requirements.txt
	@echo "... setting up virtualenv"
	python3 -m venv $(VENVDIR)
	. $(VENV); pip install --require-virtualenv \
	    --upgrade -r $(SPHINXDIR)/requirements.txt \
            --log $(VENVDIR)/pip_install.log
	@test ! -f $(VENVDIR)/pip_list.txt || \
            mv $(VENVDIR)/pip_list.txt $(VENVDIR)/pip_list.txt.bak
	@. $(VENV); pip list --local --format=freeze > $(VENVDIR)/pip_list.txt
	@touch $(VENVDIR)


.PHONY:  woke-install
woke-install:
	@type woke >/dev/null 2>&1 || \
		{ echo "Installing \"woke\" snap... \n"; sudo snap install woke; }


.PHONY:  pa11y-install
pa11y-install:
	@type $(PA11Y) >/dev/null 2>&1 || { \
			echo "Installing \"pa11y\" from npm... \n"; \
			mkdir -p $(SPHINXDIR)/node_modules/ ; \
			npm install --prefix $(SPHINXDIR) pa11y; \
		}


.PHONY:  install
install: $(VENVDIR)


.PHONY: run
run: install
	. $(VENV); sphinx-autobuild -b dirhtml "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS)


# Doesn't depend on $(BUILDDIR) to rebuild properly at every run.
.PHONY: html
html: install
	. $(VENV); $(SPHINXBUILD) -b dirhtml "$(SOURCEDIR)" "$(BUILDDIR)" \
		-w $(SPHINXDIR)/warnings.txt $(SPHINXOPTS)


.PHONY: epub
epub: install
	. $(VENV); $(SPHINXBUILD) -b epub "$(SOURCEDIR)" "$(BUILDDIR)" \
		-w $(SPHINXDIR)/warnings.txt $(SPHINXOPTS)


.PHONY: serve
serve: html
	cd "$(BUILDDIR)"; python3 -m http.server 8000


.PHONY: clean
clean: clean-doc
	@test ! -e "$(VENVDIR)" -o -d "$(VENVDIR)" -a \
		"$(abspath $(VENVDIR))" != "$(VENVDIR)"
	rm -rf $(VENVDIR)
	rm -f $(SPHINXDIR)/requirements.txt
	rm -rf $(SPHINXDIR)/node_modules/


.PHONY: clean-doc
clean-doc:
	git clean -fx "$(BUILDDIR)"
	rm -rf $(SPHINXDIR)/.doctrees


.PHONY: spelling
spelling: html
	. $(VENV) ; python3 -m pyspelling -c $(SPHINXDIR)/spellingcheck.yaml -j $(shell nproc)


.PHONY: linkcheck
linkcheck: install
	. $(VENV) ; $(SPHINXBUILD) -c . -b linkcheck  "$(SOURCEDIR)" "$(BUILDDIR)"


.PHONY: woke
woke: woke-install
	woke *.rst **/*.rst --exit-1-on-failure \
	    -c https://github.com/canonical/Inclusive-naming/raw/main/config.yml

.PHONY: pa11y
pa11y: pa11y-install html
	find $(BUILDDIR) -name *.html -print0 | xargs -n 1 -0 $(PA11Y)

# Catch-all target: route all unknown targets to Sphinx using the new
# "make mode" option.  $(O) is meant as a shortcut for $(SPHINXOPTS).
.PHONY: Makefile
%: Makefile
	. $(VENV); $(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)
