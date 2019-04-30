ifeq ($(shell which lessc),/usr/bin/lessc)
	LESSC=/usr/bin/lessc
else ifeq ($(shell which lessc),/usr/local/bin/lessc)
	LESSC=/usr/local/bin/lessc
else
	LESSC=node_modules/.bin/lessc
endif
export LESSC

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
