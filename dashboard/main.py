import time
import streamlit as st
import paho.mqtt.client as mqtt
import plotly.express as px
import plotly.graph_objects as go
import numpy as np
import paho.mqtt.publish as publish


# set page full width
st.set_page_config(layout="wide")
st.title('Aerodome Control System')

# default SERIAL_NUMBER AERODOME/3927-2849-2930-1289

device_clients = {}

sidepanel = st.sidebar
sidepanel.title('Control Panel')

sidepanel_controllers = sidepanel.empty()


fanEnabled = 0
wateringEnabled = 0

max_history = 20
temps = [{"timestamp":time.time()-(max_history-i), "value":0} for i in range(max_history)]
humidities = [{"timestamp":time.time()-(max_history-i), "value":0} for i in range(max_history)]
currentTemp = 0
currentHumidity = 0


# create two columns

gauges = st.empty()

# create chart for temperature
chart_temp_panel = st.empty()
chart_humidity_panel = st.empty()

def get_time_from_timestamp(timestamp):
    return time.strftime('%H:%M:%S', time.localtime(timestamp))

def on_connect(client, userdata, flags, rc, prop):
    print('Connected with result code '+str(rc))
    # listen to messages
    channel = serial_num_input+"/sensors";
    client.subscribe(channel)
    print('Subscribed to:', channel)

def on_message(client, userdata, msg):
    print(msg.topic+" "+str(msg.payload))
    data_tokens = msg.payload.decode('utf-8').split(',')
    temp = float(data_tokens[1])
    humidity = float(data_tokens[2])
    fanEnabled = 1-int(data_tokens[3])
    waterningEnabled = 1-int(data_tokens[4])
    if len(temps) > max_history:
        temps.pop(0)
    if len(humidities) > max_history:
        humidities.pop(0)

    temps.append({
        "timestamp": time.time(),
        "value": temp
    })
    humidities.append({
        "timestamp": time.time(),
        "value": humidity
    })

    currentTemp = temp
    currentHumidity = humidity

    with chart_temp_panel:
        fig_temp = px.line(x=[get_time_from_timestamp(x["timestamp"]) for x in temps], y=[x["value"] for x in temps], title='Temperature', labels={'x': 'Time', 'y': 'Temperature (C)'})
        # plot full width
        st.plotly_chart(fig_temp, use_container_width=True)

    with chart_humidity_panel:
        fig_humidity = px.line(x=[get_time_from_timestamp(x["timestamp"]) for x in humidities], y=[x["value"] for x in humidities], title='Humidity', labels={'x': 'Time', 'y': 'Humidity (%)'})
        st.plotly_chart(fig_humidity, use_container_width=True)


    with gauges:
        fig = go.Figure()
        fig.add_trace(go.Indicator(
            mode = "number+delta+gauge",
            value = currentTemp,
            title = {'text': "Temperature"},
            number = {"suffix": "℃"},
            delta = {'reference': np.average([x["value"] for x in temps])},
            domain = {'row': 0, 'column': 0},
            gauge= {'axis': {'range': [None, 50]}}
            ))

    

        fig .add_trace(go.Indicator(
            mode = "number+delta+gauge",
            value = currentHumidity,
            number = {"suffix": "%"},
            title = {'text': "Humidity"},
            delta = {'reference': np.average([x["value"] for x in humidities])},
             domain = {'row': 0, 'column': 1},
            gauge= {'axis': {'range': [None, 100]}}
            ))
        
        fig.update_layout(grid = {'rows': 1, 'columns': 2, 'pattern': "independent"})

        st.plotly_chart(fig, use_container_width=True)




# cache by serial number
@st.cache(allow_output_mutation=True)
def get_device(serial_number):
    return device_clients.get(serial_number, None)


serial_num_input = sidepanel.text_input('Enter Device Serial Number')
connect_button = sidepanel.button('Connect')

_client = None

if connect_button:
    st.write('Connecting to device with serial number:', serial_num_input)
    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
 
    st.write('Device connected successfully!')
    device_clients[serial_num_input] = client

    
    client.on_message = on_message
    client.on_connect = on_connect
    client.connect('broker.emqx.io')
    _client = client
    client.loop_forever()


with sidepanel_controllers:
    fanToggle = sidepanel.toggle('Fan', True if fanEnabled == 1 else False)
    wateringToggle = sidepanel.toggle('Watering', True if wateringEnabled == 1 else False)

    if fanToggle:
        publish.single(serial_num_input, "0"+str(1-fanEnabled), hostname="broker.emqx.io")
    if wateringToggle:
        publish.single(serial_num_input, "1"+str(1-wateringEnabled), hostname="broker.emqx.io")