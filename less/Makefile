CSSDIR=../static/css/

all :
	ifeq (,$(shell command -v lessc 2> /dev/null))
	$(error "less is not installed, please run install-less.sh")
	endif
	lessc icons.less --clean-css="--s1 --advanced" $(CSSDIR)icons.css

install :
	./install-less.sh
	$(MAKE) all

clean :
	rm -f write.css
	rm -f fonts.css
	rm -f icons.css
