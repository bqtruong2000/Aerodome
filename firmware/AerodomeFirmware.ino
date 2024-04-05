#include <SHT2x.h>
#include "Wire.h"
#include <MQTT.h>
// #include <CustomJWT.h>
#include <WiFiManager.h>          

WiFiManager wifiManager;
WiFiClient wifiClient;
MQTTClient mqttClient;

SHT2x sht;

bool wateringDevice = false;
bool fanDevice = false;

const int WATERING_PIN = 12;
const int FAN_PIN = 13;

char sensorChannel[40];


#define UNIQUE_SERIAL_NUMBER "AERODOME/3927-2849-2930-1289"

void configModeCallback (WiFiManager *myWiFiManager) {
  Serial.println("Entered config mode");
  Serial.println(WiFi.softAPIP());
  Serial.println(myWiFiManager->getConfigPortalSSID());
}

void messageReceived(String &topic, String &payload) {
  Serial.println("incoming: " + topic + " - " + payload);

  if(payload.length() == 2) {
    if (payload[0]=='0'){
      if(payload[1]=='0') {
        Serial.println("SELECT watering device -> OFF");
        wateringDevice = true;
      } else {
        Serial.println("SELECT waterning device -> ON");
        wateringDevice = false;
      }
    }
    if (payload[0]=='1'){
      if(payload[1]=='0') {
        Serial.println("SELECT fan -> OFF");
        fanDevice = true;
      } else {
        Serial.println("SELECT fan -> ON");
        fanDevice = false;
      }
    }
  }

  // Note: Do not use the client in the callback to publish, subscribe or
  // unsubscribe as it may cause deadlocks when other things arrive while
  // sending and receiving acknowledgments. Instead, change a global variable,
  // or push to a queue and handle it in the loop after calling `client.loop()`.
}

void connect() {
  Serial.print("checking wifi...");
  while (WiFi.status() != WL_CONNECTED) {
    Serial.print(".");
    delay(1000);
  }

  Serial.print("\nconnecting...");
  while (!mqttClient.connect("arduino", "public", "public")) {
    Serial.print(".");
    delay(1000);
  }

  Serial.println("\nconnected!");

  bool connected = mqttClient.subscribe(UNIQUE_SERIAL_NUMBER);
  if(!connected) {
    Serial.println("Cannot subscribe to MQTT");
   
  } else {
    Serial.printf("Connect to %s successfully\n",UNIQUE_SERIAL_NUMBER);
  }
  // client.unsubscribe("/hello");
}


void setup() {

  Serial.begin(115200);
  Serial.setDebugOutput(true);  
  delay(3000);
  Serial.println("Aerodome Firmware booting...");

  wifiManager.setAPCallback(configModeCallback);
  wifiManager.setConfigPortalTimeout(20);

  char *mqttServer = "broker.emqx.io";

  // id/name, placeholder/prompt, default, length
  WiFiManagerParameter mqttServerParam("server", "mqtt server", mqttServer, 40);
  wifiManager.addParameter(&mqttServerParam);

  char *secretKey = "qwerty123456";
  WiFiManagerParameter secretKeyParam("secretKey", "Secret Key", secretKey, 12);
  wifiManager.addParameter(&secretKeyParam);

  if(!wifiManager.autoConnect("Aerodome", "admin@123")) {
    Serial.println("failed to connect and hit timeout");
    delay(1000);
  } else {
    Serial.println("Connect WiFi successfully");
    Serial.print("Stored SSID: ");
    Serial.println(wifiManager.getWiFiSSID());
    Serial.print("Stored passphrase: ");
    Serial.println(wifiManager.getWiFiPass());

    WiFi.begin(wifiManager.getWiFiSSID(), wifiManager.getWiFiPass());

    mqttClient.begin(mqttServer, wifiClient);
    mqttClient.onMessage(messageReceived);

    connect();

    delay(1000);
  }

  Wire.begin();
  sht.begin();
  uint8_t stat = sht.getStatus();
  Serial.print(stat, HEX);
  Serial.println();

  sprintf(sensorChannel, "%s/%s", UNIQUE_SERIAL_NUMBER, "sensors");
  pinMode(WATERING_PIN, OUTPUT);
  pinMode(FAN_PIN, OUTPUT);
}

void loop() {
  mqttClient.loop();
  if (!mqttClient.connected()) {
    connect();
  }
  sht.read();
  Serial.print("\t");
  Serial.print(sht.getTemperature(), 1);
  Serial.print("\t");
  Serial.println(sht.getHumidity(), 1);
  char buffer[40];
  sprintf(buffer, "1,%f,%f,%d,%d", sht.getTemperature(), sht.getHumidity(), wateringDevice, fanDevice);
  mqttClient.publish(sensorChannel,buffer);

  digitalWrite(WATERING_PIN,wateringDevice);
  digitalWrite(FAN_PIN, fanDevice);

  delay(1000);
}
