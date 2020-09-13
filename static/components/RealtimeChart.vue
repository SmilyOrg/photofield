<script>
module.exports = {
  props: ["chartData", "values", "axisLabel", "scale", "postfix"],
  extends: VueChartJs.Line,
  mixins: [VueChartJs.mixins.reactiveProp],
  mounted() {
    this.renderChart(this.chartData, {
      title: {
        display: false,
        text: "",
      },
      scales: {
        xAxes: [
          {
            type: "realtime",
            realtime: {
              // onRefresh: function (chart) {
              //   chart.data.datasets.forEach(function (dataset) {
              //     dataset.data.push({
              //       x: Date.now(),
              //       y: Math.random()
              //     });
              //   });
              // },
              delay: 2000,
            },
          },
        ],
        yAxes: [
          {
            type: "linear",
            ticks: {
              suggestedMin: 0,
              suggestedMax: 1,
              // Include a dollar sign in the ticks
              callback: (value, index, values) => {
                return value / (this.scale || 1) + (this.postfix || "");
              },
            },
            scaleLabel: {
              display: true,
              labelString: this.axisLabel,
            },
          },
        ],
      },
      plugins: {
        datalabels: {
          backgroundColor: function (context) {
            return context.dataset.backgroundColor;
          },
          borderRadius: 4,
          clip: true,
          color: "white",
          font: {
            weight: "bold",
          },
          formatter: function (value) {
            return value.y;
          },
        },
      },
    });

    // this.updateDatasets(this.datasets);

    // this.updateValues(this.values)
  },
  watch: {
    chartData(chartData) {},
    values(values) {
      this.updateValues(values);
    },
    // datasets(datasets) {
    //   this.updateDatasets(this.datasets);
    //   // console.log(datasets)
    // }
  },
  methods: {
    updateValues(values) {
      const chart = this.$data._chart;
      const datasets = chart.data.datasets;
      values.forEach((value) => {
        const dataset = datasets.find(
          (dataset) => dataset.label == value.label
        );
        if (!dataset) return;
        dataset.data.push({
          x: Date.now(),
          y: value.value,
        });
      });
      chart.update({
        preservation: true,
      });

      // this.$data._chart.data.datasets.forEach(dataset => {
      //   dataset.data.push({
      //     x: Date.now(),
      //     y: values[index],
      //   })
      // });
    },
  },
};
</script>

<style scoped>
</style>
