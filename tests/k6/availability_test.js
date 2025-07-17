import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Counter, Rate } from 'k6/metrics';

// Custom metrics untuk monitoring yang lebih baik
const registrationErrors = new Counter('registration_errors');
const loginErrors = new Counter('login_errors');
const availabilityErrors = new Counter('availability_errors');
const successfulFlows = new Counter('successful_complete_flows');

// Array untuk menyimpan data yang perlu dihapus
let createdPsychologists = [];

// --- Opsi Test dengan peningkatan ---
export const options = {
  stages: [
    { duration: '5s', target: 5 },
    { duration: '10s', target: 5 },
    { duration: '5s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<800'],
    'group_duration{group:::Admin registers psychologist}': ['p(95) < 500'],
    'group_duration{group:::Psychologist sets availability}': ['p(95) < 500'],
    registration_errors: ['count<5'],
    login_errors: ['count<5'],
    successful_complete_flows: ['count>0'],
  },
};

const BASE_URL = 'http://localhost:8081';

// --- Fungsi Setup ---
export function setup() {
  const adminEmail = 'admin@example.com';
  const adminPassword = 'adminpassword';

  console.log(`Attempting to log in as admin: ${adminEmail}`);
  const loginPayload = JSON.stringify({ email: adminEmail, password: adminPassword });
  const loginParams = {
    headers: { 'Content-Type': 'application/json' },
    timeout: '10s',
  };

  const res = http.post(`${BASE_URL}/auth/login`, loginPayload, loginParams);

  if (res.status !== 200) {
    console.error(`Admin login failed with status ${res.status}: ${res.body}`);
    throw new Error(`Could not log in as admin. Status: ${res.status}. Please ensure '${adminEmail}' exists with the correct password.`);
  }

  const responseData = res.json();
  if (!responseData.data || !responseData.data.token) {
    console.error(`Admin login response missing token: ${res.body}`);
    throw new Error(`Admin login response missing token structure`);
  }

  const adminToken = responseData.data.token;
  console.log('Admin token obtained successfully for the test run.');

  // Initialize cleanup array
  createdPsychologists = [];

  return { adminToken: adminToken };
}

// --- Fungsi Utama (Default) ---
export default function (data) {
  if (!data.adminToken) {
    console.error('Skipping VU execution: Admin token was not provided from setup function.');
    return;
  }

  const timestamp = Date.now();
  const uniqueifier = `${__VU}_${__ITER}_${timestamp}`;
  const psychologistCreds = {
    email: `psikolog_${uniqueifier}@test.k6.io`,
    password: 'password123',
    username: `psikolog_${uniqueifier}`,
  };

  let psychologistToken = null;
  let registrationSuccessful = false;
  let psychologistId = null;

  // Grup 1: Admin mendaftarkan psikolog
  group('Admin registers psychologist', function () {
    const registerPsyPayload = JSON.stringify(psychologistCreds);
    const registerPsyParams = {
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${data.adminToken}`,
      },
      timeout: '10s',
    };

    const res = http.post(`${BASE_URL}/api/admin/register-psychologist`, registerPsyPayload, registerPsyParams);

    registrationSuccessful = check(res, {
      'Psychologist registered successfully': (r) => r.status === 201,
      'Registration response has success flag': (r) => {
        try {
          const body = r.json();
          return body.success === true;
        } catch (e) {
          console.error('Failed to parse registration response:', r.body);
          return false;
        }
      },
      'Registration response has user data': (r) => {
        try {
          const body = r.json();
          return body.data && body.data.username && body.data.email;
        } catch (e) {
          return false;
        }
      },
    });

    // Simpan data psikolog yang berhasil dibuat untuk cleanup
    if (registrationSuccessful) {
      try {
        const responseData = res.json();
        psychologistId = responseData.data.id || responseData.data.user_id;

        // Simpan data untuk cleanup
        createdPsychologists.push({
          id: psychologistId,
          email: psychologistCreds.email,
          username: psychologistCreds.username,
          adminToken: data.adminToken,
        });
      } catch (e) {
        console.error('Failed to extract psychologist ID:', e);
      }
    } else {
      console.error(`Registration failed for ${psychologistCreds.email}: Status ${res.status}, Body: ${res.body}`);
      registrationErrors.add(1);
    }
  });

  sleep(1);

  // Grup 2: Login sebagai psikolog yang baru dibuat
  if (registrationSuccessful) {
    group('Psychologist login', function () {
      const loginPayload = JSON.stringify({
        email: psychologistCreds.email,
        password: psychologistCreds.password,
      });
      const loginParams = {
        headers: { 'Content-Type': 'application/json' },
        timeout: '10s',
      };

      const loginRes = http.post(`${BASE_URL}/auth/login`, loginPayload, loginParams);

      const loginSuccessful = check(loginRes, {
        'Psychologist login successful': (r) => r.status === 200,
        'Login response has token': (r) => {
          try {
            const body = r.json();
            return body.data && body.data.token;
          } catch (e) {
            console.error('Failed to parse login response:', r.body);
            return false;
          }
        },
      });

      if (loginSuccessful) {
        try {
          psychologistToken = loginRes.json('data.token');
        } catch (e) {
          console.error('Failed to extract token from login response:', loginRes.body);
          loginErrors.add(1);
        }
      } else {
        console.error(`Login failed for ${psychologistCreds.email}: Status ${loginRes.status}, Body: ${loginRes.body}`);
        loginErrors.add(1);
      }
    });
  }

  // Grup 3: Set availability (hanya jika login berhasil)
  if (psychologistToken) {
    group('Psychologist sets availability', function () {
      const availabilityPayload = JSON.stringify({
        slots: [
          { hari: 'Senin', waktu_mulai: '09:00:00', waktu_selesai: '12:00:00' },
          { hari: 'Selasa', waktu_mulai: '14:00:00', waktu_selesai: '17:00:00' },
        ],
      });
      const availabilityParams = {
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${psychologistToken}`,
        },
        timeout: '10s',
      };

      const res = http.post(`${BASE_URL}/api/psychologist/availability`, availabilityPayload, availabilityParams);

      const availabilitySuccessful = check(res, {
        'Availability set successfully': (r) => r.status === 200,
        'Availability response structure': (r) => {
          try {
            const body = r.json();
            return body.success === true;
          } catch (e) {
            console.error('Failed to parse availability response:', r.body);
            return false;
          }
        },
      });

      if (availabilitySuccessful) {
        successfulFlows.add(1);
      } else {
        console.error(`Availability setting failed for ${psychologistCreds.email}: Status ${res.status}, Body: ${res.body}`);
        availabilityErrors.add(1);
      }
    });
  } else {
    console.error(`Skipping availability setting: No valid token for ${psychologistCreds.email}`);
  }

  sleep(2);
}

// --- Fungsi Cleanup Helper ---
function cleanupPsychologist(psychologist) {
  console.log(`Cleaning up psychologist: ${psychologist.email}`);

  // Opsi 1: Hapus melalui endpoint admin (jika ada)
  const deleteParams = {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${psychologist.adminToken}`,
    },
    timeout: '10s',
  };

  // Coba beberapa endpoint yang mungkin ada
  const possibleEndpoints = [`/api/admin/psychologist/${psychologist.id}`, `/api/admin/users/${psychologist.id}`, `/api/admin/delete-psychologist/${psychologist.id}`, `/api/psychologist/${psychologist.id}`];

  for (const endpoint of possibleEndpoints) {
    const res = http.del(`${BASE_URL}${endpoint}`, null, deleteParams);

    if (res.status === 200 || res.status === 204) {
      console.log(`Successfully deleted psychologist ${psychologist.email} via ${endpoint}`);
      return true;
    } else if (res.status === 404) {
      console.log(`Psychologist ${psychologist.email} not found (may already be deleted)`);
      return true;
    }
  }

  // Opsi 2: Jika tidak ada endpoint delete, coba disable user
  const disablePayload = JSON.stringify({ active: false, status: 'disabled' });
  const disableRes = http.put(`${BASE_URL}/api/admin/psychologist/${psychologist.id}`, disablePayload, deleteParams);

  if (disableRes.status === 200) {
    console.log(`Successfully disabled psychologist ${psychologist.email}`);
    return true;
  }

  console.error(`Failed to cleanup psychologist ${psychologist.email}`);
  return false;
}

// --- Fungsi Teardown dengan Cleanup ---
export function teardown(data) {
  console.log('Starting cleanup process...');

  // Cleanup semua psikolog yang dibuat
  let cleanupCount = 0;
  for (const psychologist of createdPsychologists) {
    if (cleanupPsychologist(psychologist)) {
      cleanupCount++;
    }
  }

  console.log(`Cleanup completed. Successfully cleaned up ${cleanupCount} out of ${createdPsychologists.length} psychologists.`);

  // Safe metric access helper
  const getMetricCount = (metricName) => {
    return data.metrics?.[metricName]?.values?.count || 0;
  };

  console.log('Test completed. Summary of custom metrics:');
  console.log(`- Registration errors: ${getMetricCount('registration_errors')}`);
  console.log(`- Login errors: ${getMetricCount('login_errors')}`);
  console.log(`- Availability errors: ${getMetricCount('availability_errors')}`);
  console.log(`- Successful complete flows: ${getMetricCount('successful_complete_flows')}`);
  console.log(`- Total psychologists created: ${createdPsychologists.length}`);
  console.log(`- Psychologists cleaned up: ${cleanupCount}`);
}

// --- Custom Summary Handler ---
export function handleSummary(data) {
  const getMetricValue = (metricName, property = 'count') => {
    return data.metrics?.[metricName]?.values?.[property] || 0;
  };

  const summary = {
    'Test Summary': {
      'Total Requests': getMetricValue('http_reqs'),
      'Failed Requests': getMetricValue('http_req_failed'),
      'Average Response Time': `${getMetricValue('http_req_duration', 'avg').toFixed(2)}ms`,
      '95th Percentile Response Time': `${getMetricValue('http_req_duration', 'p(95)').toFixed(2)}ms`,
      'Registration Errors': getMetricValue('registration_errors'),
      'Login Errors': getMetricValue('login_errors'),
      'Availability Errors': getMetricValue('availability_errors'),
      'Complete Successful Flows': getMetricValue('successful_complete_flows'),
      'Total Psychologists Created': createdPsychologists.length,
    },
  };

  return {
    stdout: JSON.stringify(summary, null, 2),
    'summary.json': JSON.stringify(data, null, 2),
  };
}
