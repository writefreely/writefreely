
all : local

install : 
	cd less/; $(MAKE) install $(MFLAGS)

clean :
	cd less/; $(MAKE) install $(MFLAGS)
	
local : force_look
	cd less/; $(MAKE) $(MFLAGS)
	
force_look : 
	true
