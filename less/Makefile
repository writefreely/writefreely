LESSC=$(shell command -v lessc 2> /dev/null)

ifndef LESSC
	$(error "less is not installed, please run install-less.sh")
endif

CSSDIR=../static/css/

all :
	$(LESSC) app.less --clean-css="--s1 --advanced" $(CSSDIR)write.css
	$(LESSC) fonts.less --clean-css="--s1 --advanced" $(CSSDIR)fonts.css
	$(LESSC) icons.less --clean-css="--s1 --advanced" $(CSSDIR)icons.css

install :
	./install-less.sh
	$(MAKE) all

clean :
	rm -f write.css
	rm -f fonts.css
	rm -f icons.css
