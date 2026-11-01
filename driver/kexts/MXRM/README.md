# MXRM

The kernel extension that owns the PCI function of the board: the BARs, the DMA, the interrupts, the GSP, the memory of the video card, the channels, the fences, the power and the reset. It is the only component that touches the hardware. The `-mxoff` boot argument stops it before any write to the board.
