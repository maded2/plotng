# PlotNG - plotting utility for Chia.Net

This utility consisted of server backend and UI which manages the chia plot creation.  
It uses the chia command line interface to start the plot.  
It will schedule new plots when a plot finishes as specified by the configuration file.
The server backend does a cycle every minute and check if the configuration file has been changed, if it detects that it has been changed then it reloads the configuration file.
Once a valid configuration file has been loaded then it will start one new plot per cycle.


## Installation

    go get github.com/maded2/plotng
    cd plotng
    go install plotng/cmd/plotng

Please note that I've not tested this on Windows / Mac.  (although one user confirmed it is working on MacOS)
Pre-built binaries for 64-bits Windows/MacOS/Linux included in the release

## Running Server (runs on the plotter)

`
plotng -config <json config file> -port <plotter port number, default: 8484>
`

**Please note**: chia enviornment should be activated before starting plotng

## Running UI (run anyway)

![PlotNG UI](plotng.png)

The UI can run on any host and point back to the server using the host and port parameter


`
plotng -ui -host <plotter host name or IP> -port <plotter port number, default: 8484>
`

## Configuration File (JSON format)


    {
        "Fingerprint": "",
        "FarmerPublicKey": "",
        "PoolPublicKey": "",
        "Threads": 0,
        "Buffers": 0,
        "NumberOfParallelPlots": 3,
        "TempDirectory": ["/media/eddie/plot1", "/media/eddie/plot2", , "/media/eddie/plot3"],
        "TargetDirectory": ["/media/eddie/dst1", "/media/eddie/dst2"],
        "StaggeringDelay": 30,
        "ShowPlotLog": false,
        "DiskSpaceCheck": false
    }

### Settings

- Fingerprint : fingerprint passed to the chia command line tool (you can either use the fingerprint if the private has been installed on the plotter or use the following farmer/pool public key instead)
- FarmerPublicKey : Farmer Public Key passed to the chia command line tool
- PoolPublicKey : Pool Public Key passed to the chia command line tool
- Threads : number of threads use by the chia command line tool.  If the value is zero or missing then chia will use the default
- Buffers : number of buffers use by the chia command line tool.  If the value is zero or missing then chia will use the default
- NumberOfParallelPlots : number of parallel plots to create.  Set to zero for orderly shutdown
- TempDirectory : list of plot directories / drives.  The server process will choose the next directory path on the list and wraps to the beginning when it reaches the end.
- TargetDirectory : list destination directories / drives.  The server process will choose the next directory path on the list and wraps to the beginning when it reaches the end.
- StaggeringDelay : when the TargetDirectory wraps to the beginning, it will delays the next plot create by the specified minutes.
- ShowPlotLog : shows the last 10 lines of the plot logs in the server log output.
- DiskSpaceCheck : check if destination directories have enough disk space to hold a new plot (only tested on Linux, may not work on MacOS / Windows)

Please note PlotNG now skips any destination directory which have less than 105GB is space.
