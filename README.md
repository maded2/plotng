Plotting Utility for Chia.Net

Installation

`go install plotng/cmd/plotng`

Running

`nohup ./plotng -config config.json > plotng.log &
`

Configuration File

`
{
    "Fingerprint": "636105213",
    "NumberOfParallelPlots": 3,
    "TempDirectory": ["/media/eddie/plot1", "/media/eddie/plot2", , "/media/eddie/plot3"],
    "TargetDirectory": ["/media/eddie/dst1", "/media/eddie/dst2"],
    "StaggeringDelay": 30,
    "ShowPlotLog": false
}

`