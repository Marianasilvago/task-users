
# User Matching API

This API filters users within a specific radius based on their latitude and longitude using the Haversine formula to calculate the distance between two points on Earth.

## Features

- Filters users by their geographic location, calculating the distance between two points using latitude and longitude.
- Allows setting a radius in kilometers to determine which users are within the desired distance.
- Supports basic filtering by a target user's coordinates.



## API Endpoints

### POST `/users/filter`

This endpoint filters users based on a given target user’s location (latitude and longitude) and a radius (in kilometers). It returns the list of users within that radius.

#### Request

- **Method**: `POST`
- **URL**: `/users/filter`
- **Body** (JSON):

   ```code

curl -X POST http://localhost:8080/recommendations/1 -H "Content-Type: application/json" -d '{
    "looking_for_gender": "female",
    "looking_for_diet_type": "vegan",
    "age_range": {"min": 20, "max": 30},
    "max_distance": 50
}'
   ```

#### Response

- **200 OK**:

   ```json
  [{"id":4,"name":"Diana","gender":"female","latitude":40.8501,"longitude":-73.8662,"diet_type":"vegan","age":28,"likes_sent":null,"likes_received":null,"matches_sent":null,"matches_received":null}

   ```

