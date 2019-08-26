
const selectElem = document.querySelector('select[name="collection"]');
const submitElem = document.querySelector('input[type="submit"]');
const zipInfo = document.querySelector('span.zip > ul.info');
const zipWarning = document.querySelector('span.zip > p.error');
const fileInput = document.querySelector('input[type="file"]')

document.onreadystatechange = () => {
  if ( document.readyState === "complete") {
    selectElem.disabled = true;  
    submitElem.disabled = true;
    zipInfo.hidden = true;
    zipWarning.hidden = true;
  }
}

fileInput.onchange = function() {
  if ( this.files.length === 1 ) {
    if ( this.files[0].type === 'application/zip' ) {
      selectElem.disabled = true;
      submitElem.disabled = false;
      zipInfo.hidden = false;
      zipWarning.hidden = true;
    } else if ( this.files[0].type.match('text.*')) {
      selectElem.disabled = false;
      submitElem.disabled = false;
      zipInfo.hidden = true;
      zipWarning.hidden = true;
    }
  }
  
  if ( this.files.length > 1 ) {
    selectElem.disabled = false;
    submitElem.disabled = false;
    var zips = 0;
    Array.from(this.files).forEach(file => {
        if ( file.name.endsWith(".zip") ) {
          zips++;
        }
    })
    if ( zips > 0 ) {
      zipInfo.hidden = true;
      zipWarning.hidden = false;
    } else {
      zipInfo.hidden = true;
      zipWarning.hidden = true;
    }
  }
}