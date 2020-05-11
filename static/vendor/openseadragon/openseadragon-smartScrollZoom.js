/**
 * @file OpenSeadragon plugin that allows for adjustment of zoom speed based upon the speed that the user scrolls the mouse wheel
 * @author Bassil Virk
 * @version 1.0.0
 */

 (function($){

    $.Viewer.prototype.smartScrollZoom = function (options) {
        if (!this.smartScrollZoomInstance) {
            options = options || {};
            options.viewer = this;
            this.smartScrollZoomInstance = new $.SmartScrollZoom(options);
        } else {
            this.smartScrollZoomInstance.setOptions(options);
        }
    };

    /**
     * @class SmartScrollZoom
     * @classdesc Changes zoom speed based on scrolling speed
     * @memberof OpenSeadragon
     * @param {Object} options
     * @param {Number} options.timeThreshold - Amount of time, in milliseconds, that the minimum number of scrolls must occur within 
     * before logic begins
     * @param {Number} options.minScrolls - Required number of consecutive scrolls that must take place within the specified time 
     * threshold of each other before logic begins
     * @param {Number} options.minZoomPerScroll - Minimum factor to zoom by with a single scroll. Setting this to 1 will affectively 
     * result in no zoom while logic is not being executing
     * @param {Number} options.maxZoomPerScroll - Maximum zoom factor that can be reached
     * @param {Number} options.zoomIncrement - Amount to increment zoom factor by with every scroll after minScrolls
     * @param {Boolean} options.enabled - Whether or not the scroll zoom logic is currently active
     */
    $.SmartScrollZoom = function (options) {
        //If this was not set to a viewer, throw an error
        if (!options.viewer) {
            throw new Error("SmartScrollZoom must be set to a viewer");
        }

        this.viewer = options.viewer; //Set viewer
        this.timeThreshold = options.timeThreshold || 400;
        this.minScrolls = options.minScrolls || 2;
        this.minZoomPerScroll = options.minZoomPerScroll || 1.2; //OpenSeadragon has a default of 1.2
        this.maxZoomPerScroll = options.maxZoomPerScroll || 2.5;
        this.zoomIncrement = options.zoomIncrement || 0.2;
        this.enabled = options.enabled ? true : false;

        var self = this;

        //Create handler for logic
        this.viewer.addHandler("canvas-scroll", function () {
            //Do nothing if not enabled
            if (!self.enabled) {
                return;
            }

            //Create var to count number of consecutive scrolls that have taken place within the specified time limit of each other
            if (typeof self.scrollNum == 'undefined') {
                self.scrollNum = 0;
            }
    
            //Create var to store the time of the previous scroll that occurred
            if (typeof self.lastScroll == 'undefined') {
                self.lastScroll = new Date();
            }
    
            self.currentScroll = new Date(); //Time that this scroll occurred at
    
            //If the last scroll was less than 400 ms ago, increase the scroll count
            if (self.currentScroll - self.lastScroll < self.timeThreshold) {
                self.scrollNum++;
            }
            //Otherwise, reset the count and zoom speed
            else {
                self.scrollNum = 0;
                self.viewer.zoomPerScroll = self.minZoomPerScroll;
            }
    
            //If user has scrolled more than twice consecutively within 400 ms, increase the scroll speed with each consecutive scroll afterwards
            if (self.scrollNum > self.minScrolls) {
                //Limit maximum scroll speed to 2.5
                if (self.viewer.zoomPerScroll <= self.maxZoomPerScroll) {
                    self.viewer.zoomPerScroll += self.zoomIncrement;
                }
            }
    
            self.lastScroll = self.currentScroll; //Set last scroll to now
        });
    };

    $.SmartScrollZoom.prototype = {
        
        /**
         * Set new options
         * 
         * @function
         * @memberof OpenSeadragon.SmartScrollZoom
         * @since 1.0.0
         * @version 1.0.0
         * @param {Object} options 
         */
        setOptions: function (options) {
            
            //If no new options were specifed, do nothing
            if (!options) {
                return;
            }

            //Set time threshold
            if (options.timeThreshold !== undefined) {
                this.timeThreshold = options.timeThreshold;
            }

            //Set minimum scroll number
            if (options.minScrolls !== undefined) {
                this.minScrolls = options.minScrolls;
            }

            //Set minimum zoom per scroll
            if (options.minZoomPerScroll !== undefined) {
                this.minZoomPerScroll = options.minZoomPerScroll;
            }

            //Set maximum zoom per scroll
            if (options.maxZoomPerScroll !== undefined) {
                this.maxZoomPerScroll = options.maxZoomPerScroll;
            }

            //Set zoom increment
            if (options.zoomIncrement !== undefined) {
                this.zoomIncrement = options.zoomIncrement;
            }
        },

        /**
         * Toggle the enabled option
         * 
         * @function
         * @memberof OpenSeadragon.SmartScrollZoom
         * @since 1.0.0
         * @version 1.0.0
         */
        toggleEnable: function () {
            this.enabled = !this.enabled;
        }
    };
 })(OpenSeadragon);