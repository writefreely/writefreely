ifeq (,$(shell command -v lessc 2> /dev/null))
$(error "less is not installed, please run install-less.sh")
endif

CSSDIR=../static/css/

all :
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
