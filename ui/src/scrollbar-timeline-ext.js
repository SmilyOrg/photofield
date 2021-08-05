OverlayScrollbars.extension("timeline", function(defaultOptions, framework, compatibility) { 
  const osInstance = this;
  const extension = { };
  
  let root;
  let label;
  
  extension.added = function() { 
    const instanceElements = osInstance.getElements();
    const verticalHandle = instanceElements.scrollbarVertical.handle;
    const html = `<div class="scrollbar-timeline">
      <div class="scrollbar-timeline-label"></div>
    </div>`;
    
    root = framework(html);
    label = root.find(".scrollbar-timeline-label");
    
    framework(verticalHandle).append(root);
  }
  
  extension.removed = function() { 
    root.remove();
  }

  extension.setLabel = function(text) {
    label[0].innerText = text;
  }

  return extension;
});
