CSSDIR=../static/css/

all :
	@command -v lessc >/dev/null 2>&1 || { echo >&2 "less is not installed, please run install-less.sh."; exit 1; }
	lessc app.less --clean-css="--s1 --advanced" $(CSSDIR)write.css
	lessc fonts.less --clean-css="--s1 --advanced" $(CSSDIR)fonts.css
	lessc icons.less --clean-css="--s1 --advanced" $(CSSDIR)icons.css

install :
	./install-less.sh
	$(MAKE) all

clean :
	rm -f write.css
	rm -f fonts.css
	rm -f icons.css
