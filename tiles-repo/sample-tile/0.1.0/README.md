# Sample Tile 

This is sample Tile, which is actual contruct based on CDK. The sample tile will provision SQS queue and topic. The Tile spectication - [tile-spec.yaml](./tile-spec.yaml) has full definition, includes metadata, inputs, outputs, etc.

## Tile 
Sample-Tile

## Version
0.1.0

## Category
Analysis

## Input 

- queueName
>The name of SQS queue

- visibilityTimeout
>The visibility timeout to be configured on the SQS Queue, in seconds. Default 300s.

- topicName
>The name of SNS topic


## Output values

- queueName
>AWS::SQS::Queue.Name

- queueArn
>AWS::SQS::Queue.ARN

- topicName
>AWS::SNS::Topic.Name


